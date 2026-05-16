package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/fulfillment"
)

type adminShipmentPatchRequest struct {
	Carrier      string `json:"carrier"`
	TrackingCode string `json:"tracking_code"`
}

func (s *Server) handleAdminOrderShipment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	orderID := strings.TrimPrefix(r.URL.Path, "/admin/orders/")
	orderID = strings.TrimSuffix(orderID, "/shipment")
	orderID = strings.Trim(orderID, "/")
	if orderID == "" || strings.Contains(orderID, "/") {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "not found")
		return
	}

	var req adminShipmentPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid json")
		return
	}

	svc := &fulfillment.SQLService{DB: s.DB}
	sh, err := svc.UpdateTracking(r.Context(), time.Now().UTC(), domain.OrderID(orderID), req.Carrier, req.TrackingCode)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		switch err {
		case domain.ErrInvalidInput:
			status = http.StatusUnprocessableEntity
			code = "invalid_input"
		case domain.ErrNotFound:
			status = http.StatusNotFound
			code = "not_found"
		case domain.ErrConflict:
			status = http.StatusConflict
			code = "conflict"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"shipment": map[string]any{
			"ID":           sh.ID,
			"OrderID":      sh.OrderID,
			"Carrier":      sh.Carrier,
			"TrackingCode": sh.TrackingCode,
			"Status":       sh.Status,
			"LabelPath":    sh.LabelPath,
		},
	})
}
