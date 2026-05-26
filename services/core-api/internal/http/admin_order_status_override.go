package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/orders"
)

// handleAdminOrderStatusOverride is the emergency-escape admin endpoint that
// forcibly transitions an order's status, bypassing the normal lifecycle
// driven by Stripe webhooks, the NATS order.paid consumer, and the shipment
// workflow.
//
// Roadmap item 134.
//
//	POST /admin/orders/{id}/status
//	{
//	  "status": "FULFILLING",   # required, target status (case-insensitive)
//	  "reason": "nats outage"   # required, free-form short string
//	}
//
// Role: AdminRoleOps (only the most-privileged tier — this can cancel paid
// orders and bypass payment verification, so we don't want support agents
// touching it).
//
// Allowed transitions are restricted to forward moves through the lifecycle
// plus any-state→CANCELLED (see orders.AdminAllowedTransitions). Disallowed
// transitions return 409 Conflict with the list of acceptable targets in
// the error body.
//
// Side-effect contract:
//   - PENDING_PAYMENT → PAID does NOT consume inventory (use the normal
//     payment_intent.succeeded webhook or POST /admin/orders/{id}/reconcile-stripe
//     for that). This override is for stuck orders where Stripe already
//     paid but the local row is wedged.
//   - Any → CANCELLED past PENDING_PAYMENT does NOT release inventory and
//     does NOT trigger a Stripe refund. Operators must follow up with
//     POST /admin/orders/{id}/refund.
//   - Emits an order.status.overridden outbox envelope; consumers can react
//     to the override the same way they react to natural transitions.
//   - Writes an audit_events row with actor from X-Admin-Actor.
func (s *Server) handleAdminOrderStatusOverride(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Admin == nil || s.Orders == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing order id")
		return
	}
	orderID := domain.OrderID(id)

	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}
	target := domain.OrderStatus(strings.ToUpper(strings.TrimSpace(body.Status)))
	reason := strings.TrimSpace(body.Reason)
	if target == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "missing target status")
		return
	}
	if reason == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "reason is required for status override")
		return
	}

	out, err := s.Orders.AdminOverrideStatus(r.Context(), orderID, target, reason)
	if err != nil {
		switch err {
		case domain.ErrNotFound:
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
		case domain.ErrInvalidInput:
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid status or reason")
		case domain.ErrConflict:
			cur, _, getErr := s.Orders.GetOrder(r.Context(), orderID)
			detail := "transition not allowed"
			if getErr == nil && cur != nil {
				if allowed, ok := orders.AdminAllowedTransitions[cur.Status]; ok {
					targets := make([]string, 0, len(allowed))
					for _, t := range allowed {
						targets = append(targets, string(t))
					}
					detail = fmt.Sprintf("cannot override %s → %s; allowed targets: %s",
						cur.Status, target, strings.Join(targets, ", "))
				} else {
					detail = fmt.Sprintf("status %s is terminal; no override allowed", cur.Status)
				}
			}
			writeAPIError(w, r, http.StatusConflict, "conflict", detail)
		default:
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	actor := adminActorFromRequest(r)
	payload, _ := json.Marshal(map[string]any{
		"to_status": string(target),
		"reason":    reason,
	})
	if err := s.Admin.RecordAudit(r.Context(), actor, "order", string(orderID),
		"order.status.overridden", string(payload)); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	_, items, err := s.Orders.GetOrder(r.Context(), orderID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order": out,
		"items": items,
	})
}
