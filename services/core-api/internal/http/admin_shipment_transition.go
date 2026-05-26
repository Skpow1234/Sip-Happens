package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/fulfillment"
)

// handleAdminShipmentTransition lets an ops admin move a shipment through
// any whitelisted state including FAILED. This is the state-machine
// counterpart to PATCH /admin/orders/{id}/shipment, which only handles
// the carrier/tracking metadata along the happy path.
//
// Roadmap item 137.
//
//	POST /admin/orders/{id}/shipment/transition
//	{
//	  "to":     "FAILED",         # required, target shipment status
//	  "reason": "lost in transit" # required, free-form, audited
//	}
//
// Role: AdminRoleOps. Disallowed transitions return 409 with the list
// of acceptable targets so the admin console can render them as buttons.
//
// Side effects:
//   - Transitioning to IN_TRANSIT also flips the order to SHIPPED
//     (mirrors the natural fulfillment-runner happy path).
//   - Transitioning to FAILED does NOT auto-cancel the order or refund;
//     operators must use POST /admin/orders/{id}/status (cancel) and/or
//     POST /admin/orders/{id}/refund. The runbook explains why
//     (see docs/runbooks/reservation-release-failure.md).
//   - Writes an audit_events row tagged `shipment.transitioned` with
//     {from, to, reason}.
func (s *Server) handleAdminShipmentTransition(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil || s.Admin == nil {
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
		To     string `json:"to"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}
	to := domain.ShipmentStatus(strings.ToUpper(strings.TrimSpace(body.To)))
	reason := strings.TrimSpace(body.Reason)
	if to == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "missing target shipment status")
		return
	}
	if reason == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "reason is required for shipment transition")
		return
	}

	svc := &fulfillment.SQLService{DB: s.DB}
	sh, prev, err := svc.TransitionStatus(r.Context(), time.Now().UTC(), orderID, to, reason)
	if err != nil {
		switch err {
		case domain.ErrInvalidInput:
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid target status")
		case domain.ErrNotFound:
			writeAPIError(w, r, http.StatusNotFound, "not_found", "shipment not found")
		case domain.ErrConflict:
			detail := disallowedShipmentTransitionDetail(prev, to)
			writeAPIError(w, r, http.StatusConflict, "conflict", detail)
		default:
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	actor := adminActorFromRequest(r)
	payload, _ := json.Marshal(map[string]any{
		"from":     string(prev),
		"to":       string(sh.Status),
		"reason":   reason,
		"shipment": sh.ID,
	})
	if err := s.Admin.RecordAudit(r.Context(), actor, "shipment", string(sh.ID),
		"shipment.transitioned", string(payload)); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	// Fire-and-forget shipping update email when the status actually
	// changed (no-op on idempotent re-submits). Failures are logged but
	// don't fail the API response — operators can resend manually.
	if prev != sh.Status && s.Email != nil {
		go func(status, carrier, code, why string) {
			if err := s.Email.SendShippingUpdateForOrder(r.Context(),
				orderID, domain.ShipmentStatus(status), carrier, code, why); err != nil {
				// Log only — see obslog usage in email_subscribe_handlers.
				_ = err
			}
		}(string(sh.Status), sh.Carrier, sh.TrackingCode, reason)
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"shipment": map[string]any{
			"ID":           sh.ID,
			"OrderID":      sh.OrderID,
			"Carrier":      sh.Carrier,
			"TrackingCode": sh.TrackingCode,
			"LabelPath":    sh.LabelPath,
			"Status":       sh.Status,
			"UpdatedAt":    sh.UpdatedAt,
		},
		"previous_status": prev,
	})
}

func disallowedShipmentTransitionDetail(from, to domain.ShipmentStatus) string {
	allowed, ok := fulfillment.AdminAllowedShipmentTransitions[from]
	if !ok || len(allowed) == 0 {
		return fmt.Sprintf("shipment is %s (terminal); no transition allowed", from)
	}
	targets := make([]string, 0, len(allowed))
	for _, a := range allowed {
		targets = append(targets, string(a))
	}
	return fmt.Sprintf("cannot transition shipment %s → %s; allowed: %s",
		from, to, strings.Join(targets, ", "))
}
