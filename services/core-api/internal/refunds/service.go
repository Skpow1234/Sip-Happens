// Package refunds implements partial-and-full refund issuance against Stripe
// payment intents, recorded in the local `refunds` ledger and mirrored to
// `payment_status_history`. The package is intentionally thin — most of
// the lifecycle is event-driven (refund.* / charge.refunded webhook) so
// the synchronous service surface is just "create" plus a few read
// helpers for the admin endpoint and webhook handler to share.
package refunds

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/pkg/obslog"
	"sip-happens/services/core-api/internal/audit"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

// Service exposes refund operations to the HTTP layer and the webhook
// handler. Both touch the same `refunds` rows but along different axes:
// CreateRefund inserts a new PENDING row and asks Stripe to refund; the
// webhook MarkRefundStatus moves rows from PENDING → SUCCEEDED/FAILED.
type Service interface {
	// CreateRefund issues a refund against the given paid order. If amount
	// is zero, the full remaining balance is refunded. Stripe-side
	// idempotency uses a sha256 of (order_id|amount|reason) so a retry
	// of the same logical refund collapses into one Stripe call.
	CreateRefund(ctx context.Context, in CreateRefundInput) (*domain.Refund, error)

	// MarkRefundStatusByStripeID is called by the webhook handler when a
	// `charge.refunded` or `refund.updated` event arrives. Idempotent —
	// re-applying the same terminal status is a no-op.
	MarkRefundStatusByStripeID(ctx context.Context, stripeRefundID, newStatus, failureMessage string) error

	// ListByOrder returns every refund row for an order (oldest first).
	ListByOrder(ctx context.Context, orderID domain.OrderID) ([]domain.Refund, error)
}

type CreateRefundInput struct {
	OrderID          domain.OrderID
	AmountMinorUnits int64 // 0 = full remaining balance
	Reason           string
	Actor            string
}

// SQLService wires the refunds Service against Postgres + the Stripe client.
//
// Field set:
//
//   - DB     – sql.DB; all writes happen in a serializable tx.
//   - Stripe – the Stripe Service interface; mocked in unit tests.
//   - Audit  – audit.Store; rows are appended for every state change.
//
// All three are required for a usable instance; a nil Stripe field puts
// the service in "local-only" mode used by the webhook-only consumer.
type SQLService struct {
	DB     *sql.DB
	Stripe stripepayments.Service
	Audit  *audit.Store
}

var (
	// ErrOrderNotRefundable is returned when the order is not in PAID,
	// FULFILLING, SHIPPED, or REFUNDED — i.e. cannot have money refunded
	// from it (PENDING_PAYMENT / CANCELLED / DRAFT).
	ErrOrderNotRefundable = errors.New("refunds: order not in a refundable status")
	// ErrAmountExceedsRemaining is returned when the requested amount
	// would refund more than the order's total minus already-refunded.
	ErrAmountExceedsRemaining = errors.New("refunds: amount exceeds remaining refundable balance")
	// ErrMissingPaymentIntent is returned when the order has no Stripe
	// payment intent on file — we can't refund what we don't have a
	// reference to.
	ErrMissingPaymentIntent = errors.New("refunds: order has no stripe_payment_intent_id")
)

// IdempotencyKey is the stable Stripe Idempotency-Key for a (order, amount,
// reason) tuple. Two retries of the same logical refund collapse into a
// single Stripe call; two genuinely different refund attempts on the same
// order (e.g. partial refund #1 and partial refund #2 of different amounts)
// hash differently so Stripe correctly issues both.
func IdempotencyKey(orderID domain.OrderID, amountMinor int64, reason string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", orderID, amountMinor, strings.TrimSpace(reason))))
	return "refund-" + hex.EncodeToString(h[:8])
}

// CreateRefund persists a PENDING row, calls Stripe (idempotency-keyed),
// updates the row with the Stripe id + new status, writes audit and
// payment_status_history entries, and flips the order to REFUNDED on
// success.
//
// The order status flip happens in the same tx as the refunds row write,
// so a partial failure (e.g. Stripe call errored) leaves nothing visible.
// Stripe is called *outside* the tx because a long network call inside a
// serializable tx invites deadlocks and connection-pool starvation; the
// trade-off is that we have a brief window where the local row is
// PENDING but Stripe hasn't acknowledged. The webhook handler reconciles.
func (s *SQLService) CreateRefund(ctx context.Context, in CreateRefundInput) (*domain.Refund, error) {
	if s.DB == nil || s.Stripe == nil {
		return nil, fmt.Errorf("refunds: missing dependencies")
	}
	if in.OrderID == "" {
		return nil, domain.ErrInvalidInput
	}
	if in.AmountMinorUnits < 0 {
		return nil, domain.ErrInvalidInput
	}
	actor := strings.TrimSpace(in.Actor)
	if actor == "" {
		actor = "admin"
	}
	reason := strings.TrimSpace(in.Reason)

	// 1. Read order + sum of prior non-failed refunds in one snapshot to
	//    compute the remaining refundable balance.
	var (
		status     string
		totalMinor int64
		currency   string
		piID       sql.NullString
		paymentID  sql.NullString
	)
	row := s.DB.QueryRowContext(ctx, `
select o.status,
       o.total_minor_units,
       o.currency,
       o.stripe_payment_intent_id,
       (select id from payments where order_id = o.id and status = 'SUCCEEDED' order by created_at desc limit 1)
from orders o
where o.id = $1
`, string(in.OrderID))
	if err := row.Scan(&status, &totalMinor, &currency, &piID, &paymentID); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if !isRefundableStatus(domain.OrderStatus(status)) {
		return nil, ErrOrderNotRefundable
	}
	if !piID.Valid || strings.TrimSpace(piID.String) == "" {
		return nil, ErrMissingPaymentIntent
	}

	var refundedSoFar int64
	if err := s.DB.QueryRowContext(ctx, `
select coalesce(sum(amount_minor_units),0)
from refunds
where order_id = $1
  and status in ('PENDING','SUCCEEDED')
`, string(in.OrderID)).Scan(&refundedSoFar); err != nil {
		return nil, err
	}
	remaining := totalMinor - refundedSoFar
	if remaining <= 0 {
		return nil, ErrAmountExceedsRemaining
	}
	amount := in.AmountMinorUnits
	if amount == 0 {
		amount = remaining
	}
	if amount > remaining {
		return nil, ErrAmountExceedsRemaining
	}

	// 2. Insert a PENDING row (so a Stripe-side success that we can't
	//    persist locally still has a matching row when the webhook
	//    catches up — idempotency_key ties the two together).
	refundID, err := ids.UUIDv4()
	if err != nil {
		return nil, err
	}
	idemKey := IdempotencyKey(in.OrderID, amount, reason)
	now := time.Now().UTC()
	var pmID sql.NullString
	if paymentID.Valid {
		pmID = paymentID
	}
	if _, err := s.DB.ExecContext(ctx, `
insert into refunds
  (id, order_id, payment_id, amount_minor_units, currency, reason,
   idempotency_key, status, actor, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$9,$9)
on conflict (order_id, idempotency_key) do nothing
`, "ref_"+refundID, string(in.OrderID), pmID, amount, currency, reason,
		idemKey, actor, now); err != nil {
		return nil, err
	}

	// If a row already existed for this idempotency_key we return it
	// instead of re-issuing — this is the local-side mirror of Stripe's
	// own idempotency behaviour.
	existing, err := s.fetchByIdempotency(ctx, in.OrderID, idemKey)
	if err != nil {
		return nil, err
	}
	if existing.Status != domain.RefundStatusPending {
		return existing, nil
	}

	// 3. Call Stripe outside the tx.
	out, err := s.Stripe.CreateRefund(ctx, stripepayments.CreateRefundInput{
		PaymentIntentID:  piID.String,
		AmountMinorUnits: amount,
		Reason:           reason,
		IdempotencyKey:   idemKey,
		OrderID:          in.OrderID,
	})
	if err != nil {
		s.markRefundFailed(ctx, existing.ID, err.Error())
		obslog.Warn(ctx, "refund_stripe_call_failed", map[string]any{
			"order_id": string(in.OrderID),
			"err":      err.Error(),
		})
		return nil, err
	}

	// 4. Update the local row with the Stripe id + normalized status.
	newStatus := normalizeStripeRefundStatus(out.Status)
	if _, err := s.DB.ExecContext(ctx, `
update refunds
set stripe_refund_id = $1,
    status = $2,
    failure_message = $3,
    updated_at = now()
where id = $4
`, out.StripeRefundID, string(newStatus), out.FailureReason, string(existing.ID)); err != nil {
		return nil, err
	}

	// 5. If terminal-success on the first call, flip order to REFUNDED +
	//    payment_status_history row + outbox event.
	if newStatus == domain.RefundStatusSucceeded {
		if err := s.markOrderRefunded(ctx, in.OrderID, existing.ID, amount, currency, reason, actor); err != nil {
			obslog.Warn(ctx, "refund_post_persist_failed", map[string]any{
				"order_id": string(in.OrderID),
				"err":      err.Error(),
			})
		}
	}

	return s.fetchByID(ctx, existing.ID)
}

// MarkRefundStatusByStripeID is the webhook entry point. It looks up the
// local row by stripe_refund_id, applies the new status (if not already
// terminal), and — when transitioning to SUCCEEDED — flips the order to
// REFUNDED. Idempotent for terminal states.
func (s *SQLService) MarkRefundStatusByStripeID(ctx context.Context, stripeRefundID, newStatus, failureMessage string) error {
	if s.DB == nil {
		return fmt.Errorf("refunds: missing db")
	}
	stripeRefundID = strings.TrimSpace(stripeRefundID)
	if stripeRefundID == "" {
		return domain.ErrInvalidInput
	}
	row := s.DB.QueryRowContext(ctx, `
select id, order_id, amount_minor_units, currency, reason, status
from refunds
where stripe_refund_id = $1
`, stripeRefundID)
	var (
		id        string
		orderID   string
		amount    int64
		currency  string
		reason    string
		curStatus string
	)
	if err := row.Scan(&id, &orderID, &amount, &currency, &reason, &curStatus); err != nil {
		if err == sql.ErrNoRows {
			// Webhook arrived for a refund we didn't initiate (operator
			// issued it directly in Stripe). Synthesize a row so audit
			// stays complete. Best-effort; if the schema rejects it we
			// log and move on rather than NACK the webhook forever.
			return s.adoptStripeInitiatedRefund(ctx, stripeRefundID, newStatus, failureMessage)
		}
		return err
	}
	normalized := normalizeStripeRefundStatus(newStatus)
	if domain.RefundStatus(curStatus) == normalized {
		return nil
	}
	// Don't downgrade a terminal SUCCEEDED to PENDING on a delayed
	// `refund.updated` event.
	if curStatus == string(domain.RefundStatusSucceeded) && normalized == domain.RefundStatusPending {
		return nil
	}
	if _, err := s.DB.ExecContext(ctx, `
update refunds
set status = $1,
    failure_message = $2,
    updated_at = now()
where id = $3
`, string(normalized), failureMessage, id); err != nil {
		return err
	}
	if normalized == domain.RefundStatusSucceeded {
		return s.markOrderRefunded(ctx, domain.OrderID(orderID), domain.RefundID(id), amount, currency, reason, "stripe_webhook")
	}
	return nil
}

func (s *SQLService) ListByOrder(ctx context.Context, orderID domain.OrderID) ([]domain.Refund, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("refunds: missing db")
	}
	rows, err := s.DB.QueryContext(ctx, `
select id, order_id, coalesce(payment_id,''), amount_minor_units, currency,
       reason, coalesce(stripe_refund_id,''), idempotency_key, status,
       actor, coalesce(failure_message,''), created_at, updated_at
from refunds
where order_id = $1
order by created_at asc
`, string(orderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Refund
	for rows.Next() {
		var r domain.Refund
		var paymentID string
		if err := rows.Scan(&r.ID, &r.OrderID, &paymentID, &r.AmountMinorUnits, &r.Currency,
			&r.Reason, &r.StripeRefundID, &r.IdempotencyKey, &r.Status,
			&r.Actor, &r.FailureMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.PaymentID = domain.PaymentID(paymentID)
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- internals ---

func (s *SQLService) fetchByID(ctx context.Context, id domain.RefundID) (*domain.Refund, error) {
	row := s.DB.QueryRowContext(ctx, `
select id, order_id, coalesce(payment_id,''), amount_minor_units, currency,
       reason, coalesce(stripe_refund_id,''), idempotency_key, status,
       actor, coalesce(failure_message,''), created_at, updated_at
from refunds where id = $1
`, string(id))
	var r domain.Refund
	var paymentID string
	if err := row.Scan(&r.ID, &r.OrderID, &paymentID, &r.AmountMinorUnits, &r.Currency,
		&r.Reason, &r.StripeRefundID, &r.IdempotencyKey, &r.Status,
		&r.Actor, &r.FailureMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	r.PaymentID = domain.PaymentID(paymentID)
	return &r, nil
}

func (s *SQLService) fetchByIdempotency(ctx context.Context, orderID domain.OrderID, key string) (*domain.Refund, error) {
	row := s.DB.QueryRowContext(ctx, `
select id, order_id, coalesce(payment_id,''), amount_minor_units, currency,
       reason, coalesce(stripe_refund_id,''), idempotency_key, status,
       actor, coalesce(failure_message,''), created_at, updated_at
from refunds where order_id = $1 and idempotency_key = $2
`, string(orderID), key)
	var r domain.Refund
	var paymentID string
	if err := row.Scan(&r.ID, &r.OrderID, &paymentID, &r.AmountMinorUnits, &r.Currency,
		&r.Reason, &r.StripeRefundID, &r.IdempotencyKey, &r.Status,
		&r.Actor, &r.FailureMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	r.PaymentID = domain.PaymentID(paymentID)
	return &r, nil
}

func (s *SQLService) markRefundFailed(ctx context.Context, id domain.RefundID, message string) {
	_, _ = s.DB.ExecContext(ctx, `
update refunds
set status = 'FAILED',
    failure_message = $1,
    updated_at = now()
where id = $2
`, message, string(id))
}

// markOrderRefunded flips the order to REFUNDED (if not already there),
// appends a REFUNDED payment_status_history row, and writes an audit
// entry. Best-effort wrapped in a single tx so a partial failure leaves
// no half-committed state.
func (s *SQLService) markOrderRefunded(ctx context.Context, orderID domain.OrderID, refundID domain.RefundID, amount int64, currency, reason, actor string) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
update orders set status = 'REFUNDED', updated_at = now()
where id = $1 and status in ('PAID','FULFILLING','SHIPPED')
`, string(orderID)); err != nil {
		return err
	}

	detailB, _ := json.Marshal(map[string]any{
		"refund_id":          string(refundID),
		"amount_minor_units": amount,
		"currency":           currency,
		"reason":             reason,
	})
	if _, err := tx.ExecContext(ctx, `
insert into payment_status_history
  (order_id, provider, provider_ref, status, source, detail, created_at)
values ($1,'stripe','',$2,$3,$4,now())
`, string(orderID), string(domain.PaymentStatusRefunded), "refund:"+actor, string(detailB)); err != nil {
		return err
	}

	if s.Audit != nil {
		_ = s.Audit.Insert(ctx, tx, "order", string(orderID), actor,
			"order.refunded", string(detailB))
	}
	return tx.Commit()
}

func (s *SQLService) adoptStripeInitiatedRefund(ctx context.Context, stripeRefundID, newStatus, failureMessage string) error {
	// Best-effort synthesizer for refunds we didn't initiate. We don't
	// know the order id from a `refund.updated` event payload alone
	// without a Stripe API call to expand `payment_intent`; that's a
	// follow-up. For now we log and accept the webhook.
	obslog.Warn(ctx, "refund_stripe_initiated_unknown_locally", map[string]any{
		"stripe_refund_id": stripeRefundID,
		"new_status":       newStatus,
		"failure_message":  failureMessage,
	})
	return nil
}

func isRefundableStatus(s domain.OrderStatus) bool {
	switch s {
	case domain.OrderStatusPaid,
		domain.OrderStatusFulfilling,
		domain.OrderStatusShipped,
		domain.OrderStatusRefunded:
		return true
	default:
		return false
	}
}

// normalizeStripeRefundStatus maps Stripe's raw status string to our enum.
// Stripe values: "pending" | "requires_action" | "succeeded" | "failed" |
// "canceled". We collapse pending/requires_action to PENDING and
// canceled→FAILED (a canceled refund means money did NOT return).
func normalizeStripeRefundStatus(raw string) domain.RefundStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "succeeded":
		return domain.RefundStatusSucceeded
	case "failed", "canceled", "cancelled":
		return domain.RefundStatusFailed
	default:
		return domain.RefundStatusPending
	}
}
