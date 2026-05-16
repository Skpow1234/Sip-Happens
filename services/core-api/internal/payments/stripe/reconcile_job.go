package stripe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
)

type ReconcileJob struct {
	DB        *sql.DB
	Orders    orders.Service
	OrderRepo orders.Repository
	Payments  payments.Repository

	Stripe Service
	Log    EventLogStore
	Queue  *SQLReconcileQueue
}

func (j *ReconcileJob) EnqueueStalePending(ctx context.Context, now time.Time, lookback time.Duration, limit int) (int, error) {
	if j.DB == nil || j.Queue == nil {
		return 0, errors.New("missing db/queue")
	}
	cutoff := now.Add(-lookback)
	rows, err := j.DB.QueryContext(ctx, `
select id
from orders
where status='PENDING_PAYMENT'
  and updated_at <= $1
  and (stripe_payment_intent_id is not null and stripe_payment_intent_id <> '')
order by updated_at asc
limit $2
`, cutoff, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return n, err
		}
		if err := j.Queue.Upsert(ctx, domain.OrderID(id), now); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func (j *ReconcileJob) RunBatch(ctx context.Context, now time.Time, limit int) (int, error) {
	if j.Queue == nil || j.OrderRepo == nil || j.Orders == nil || j.Stripe == nil {
		return 0, errors.New("missing dependencies")
	}
	ids, err := j.Queue.ListDue(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	processed := 0
	for _, id := range ids {
		if err := j.reconcileOne(ctx, now, id); err != nil {
			if j.Queue != nil {
				_ = j.Queue.MarkAttempt(ctx, id, now.Add(2*time.Minute), err.Error())
			}
			if j.Log != nil {
				_ = j.Log.InsertDeadLetter(ctx, "", fmt.Sprintf("reconcile_failed order_id=%s", id), err.Error())
			}
			continue
		}
		processed++
		_ = j.Queue.Delete(ctx, id)
	}
	return processed, nil
}

func (j *ReconcileJob) reconcileOne(ctx context.Context, now time.Time, id domain.OrderID) error {
	o, _, err := j.Orders.GetOrder(ctx, id)
	if err != nil {
		return err
	}
	if o.Status != domain.OrderStatusPendingPayment {
		return nil
	}
	if o.StripePaymentIntentID == "" {
		return errors.New("missing stripe_payment_intent_id")
	}

	st, err := j.Stripe.PaymentIntentStatus(ctx, o.StripePaymentIntentID)
	if err != nil {
		return err
	}

	switch st {
	case "succeeded":
		payment := domain.Payment{
			Provider:    domain.PaymentProviderStripe,
			ProviderRef: o.StripePaymentIntentID,
			Status:      domain.PaymentStatusSucceeded,
			RawPayload:  fmt.Sprintf(`{"reconciled_at":"%s","stripe_payment_intent_id":"%s","stripe_status":"%s"}`, now.UTC().Format(time.RFC3339Nano), o.StripePaymentIntentID, st),
		}
		return j.Orders.MarkPaid(ctx, id, payment)
	case "processing", "requires_action", "requires_payment_method", "requires_confirmation":
		if j.Payments != nil {
			_ = j.Payments.RecordStatus(ctx, j.DB, domain.PaymentStatusHistoryEntry{
				OrderID:     id,
				Provider:    domain.PaymentProviderStripe,
				ProviderRef: o.StripePaymentIntentID,
				Status:      NormalizePaymentIntentStatus(st),
				Source:      "stripe_reconcile",
				Detail:      fmt.Sprintf(`{"stripe_status":%q}`, st),
			})
		}
		return nil
	default:
		// unknown or terminal states: keep in dead-letter for manual review
		return fmt.Errorf("unhandled stripe payment intent status: %s", st)
	}
}

