package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
)

func (s *Server) handleAdminOrderIssueInvoice(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if s.Invoices == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "invoicing not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing id")
		return
	}
	orderID := domain.OrderID(id)

	actor := adminActorFromRequest(r)
	inv, err := s.Invoices.IssueForOrder(r.Context(), orderID, actor)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
			code = "not_found"
		} else if err == domain.ErrInvalidInput {
			status = http.StatusUnprocessableEntity
			code = "invalid_input"
		}
		writeAPIError(w, r, status, code, err.Error())
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"invoice": inv})
}
