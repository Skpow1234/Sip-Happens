package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/audit"
	"sip-happens/services/core-api/internal/orders"
)

// handleCustomerCancelOrder lets a customer cancel their own order while it is
// still in PENDING_PAYMENT (i.e. before they checked out at Stripe or before
// the 15-minute expiry kicks in).
//
// Contract:
//
//   - Auth: signed customer request (HMAC). The caller's customer is resolved
//     from X-SSH-Fingerprint and the order must belong to them — cross-customer
//     attempts return 404 (we never confirm existence of someone else's order).
//   - Path: POST /orders/{ref}/cancel where {ref} is either the internal order
//     id or the display ref (SH-…).
//   - Body (optional): `{"reason": "<short code>"}`. If omitted, we record
//     "customer_cancel". The reason is threaded into the underlying
//     CancelPendingOrder so it appears on the inventory release audit row
//     AND on the order.cancelled outbox envelope.
//   - Idempotency: wrapped by idem.Wrap at the route level, so retries with
//     the same Idempotency-Key return the original response without
//     re-cancelling. The cancel itself is already idempotent at the service
//     layer (re-cancelling a CANCELLED order is a no-op).
//   - Status mapping: PENDING_PAYMENT → CANCELLED returns 200 with the
//     updated order. Anything else (PAID / FULFILLING / SHIPPED) returns
//     409 Conflict — to refund a paid order, use the admin refund endpoint.
//
// We intentionally do NOT cancel the Stripe checkout session here. Stripe
// will let it expire on its own and the webhook for checkout.session.expired
// is a no-op against an already-CANCELLED order. If the customer races and
// pays after we cancel locally, the payment_intent.succeeded webhook returns
// ErrConflict and the row stays CANCELLED — operators reconcile via
// POST /admin/orders/{id}/reconcile-stripe.
func (s *Server) handleCustomerCancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Orders == nil || s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	ref := strings.TrimSpace(r.PathValue("ref"))
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing order ref")
		return
	}

	fp := strings.TrimSpace(r.Header.Get("X-SSH-Fingerprint"))
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.Customers.IDByFingerprint(r.Context(), fp)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	// Ownership check: GetOrderForCustomer accepts internal id OR display_ref
	// and returns ErrNotFound for someone else's order — so cross-customer
	// probes don't leak existence.
	o, _, err := s.Orders.GetOrderForCustomer(r.Context(), customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "customer_cancel"
	}

	if err := s.Orders.CancelPendingOrder(r.Context(), o.ID, reason); err != nil {
		switch err {
		case domain.ErrNotFound:
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
		case domain.ErrConflict:
			writeAPIError(w, r, http.StatusConflict, "conflict", "order cannot be cancelled in current status")
		default:
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	// Customer-actor audit row. No admin Service is required here because we
	// write directly via audit.NewSQLStore — the admin path uses the same
	// underlying writer (admin.SQLService.RecordAudit -> audit.Insert).
	if s.DB != nil {
		store := audit.NewSQLStore()
		payload, _ := json.Marshal(map[string]string{
			"reason":      reason,
			"customer_id": string(customerID),
			"order_ref":   ref,
		})
		_ = store.Insert(r.Context(), s.DB, "order", string(o.ID),
			"customer:"+string(customerID), "order.cancelled.by_customer", string(payload))
	}

	out, items, err := s.Orders.GetOrder(r.Context(), o.ID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	itemViews := make([]orders.OrderItemView, 0, len(items))
	for _, it := range items {
		itemViews = append(itemViews, orders.OrderItemViewFromDomain(it))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order": orders.OrderDetailViewFromDomain(*out),
		"items": itemViews,
	})
}
