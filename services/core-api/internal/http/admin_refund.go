package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/refunds"
)

// handleAdminOrderRefund issues a partial or full refund against a paid
// order. Roadmap: Terminal.shop parity / refunds-post-pay.
//
//	POST /admin/orders/{id}/refund
//	{
//	  "amount_minor_units": 50000,   # optional; omit / zero = full remaining
//	  "reason": "fraudulent"         # optional
//	}
//
// Role: AdminRoleOps (money-out is the most sensitive admin operation).
//
// Idempotency: Two-layer.
//   - HTTP: the route is wrapped by adminRL + idem, so client retries with
//     the same Idempotency-Key return the same response.
//   - Stripe: the refunds service derives a stable
//     sha256(order_id|amount|reason) and passes it as Stripe's
//     Idempotency-Key, so repeated POSTs with NO Idempotency-Key header but
//     identical body still collapse into a single Stripe-side refund.
//
// Response: the persisted refund row (status = SUCCEEDED on first-call
// success with a mock, PENDING with the real client until the webhook
// catches up). The webhook handler reconciles via charge.refunded.
//
// Order side-effects on first non-failed refund:
//   - orders.status flips PAID/FULFILLING/SHIPPED → REFUNDED
//   - payment_status_history gets a REFUNDED row
//   - audit_events gets order.refunded with the actor + amount + reason
//   - (no outbox event yet — TypeOrdersOrderRefunded is reserved but the
//     consumer side is unimplemented; downstream consumers should
//     subscribe to it when added.)
func (s *Server) handleAdminOrderRefund(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Refunds == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "refunds service not configured")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing order id")
		return
	}
	orderID := domain.OrderID(id)

	var body struct {
		AmountMinorUnits int64  `json:"amount_minor_units"`
		Reason           string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	actor := adminActorFromRequest(r)
	out, err := s.Refunds.CreateRefund(r.Context(), refunds.CreateRefundInput{
		OrderID:          orderID,
		AmountMinorUnits: body.AmountMinorUnits,
		Reason:           strings.TrimSpace(body.Reason),
		Actor:            actor,
	})
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
		case errors.Is(err, domain.ErrInvalidInput):
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid refund parameters")
		case errors.Is(err, refunds.ErrOrderNotRefundable):
			writeAPIError(w, r, http.StatusConflict, "conflict", "order is not in a refundable status (must be PAID, FULFILLING, SHIPPED, or REFUNDED)")
		case errors.Is(err, refunds.ErrAmountExceedsRemaining):
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "amount exceeds remaining refundable balance")
		case errors.Is(err, refunds.ErrMissingPaymentIntent):
			writeAPIError(w, r, http.StatusConflict, "conflict", "order has no stripe payment intent on file; nothing to refund")
		default:
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"refund": out})
}

// handleAdminOrderRefunds lists the refund ledger for an order. Read-only,
// support-role.
//
//	GET /admin/orders/{id}/refunds
func (s *Server) handleAdminOrderRefunds(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleSupport) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Refunds == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "refunds service not configured")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing order id")
		return
	}
	list, err := s.Refunds.ListByOrder(r.Context(), domain.OrderID(id))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order_id": id,
		"refunds":  list,
	})
}
