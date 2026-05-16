package http

import (
	"fmt"
	"net/http"
	"time"

	"sip-happens/services/core-api/internal/admin"
)

func (s *Server) handleAdminOrdersExport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
		return
	}
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status, err := orderStatusFilterOrEmpty(r.URL.Query().Get("status"))
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid status filter")
		return
	}
	createdFrom, err := parseOptionalRFC3339Query(r, "created_from")
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid created_from (RFC3339)")
		return
	}
	createdTo, err := parseOptionalRFC3339Query(r, "created_to")
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid created_to (RFC3339)")
		return
	}

	limit := parseLimitQuery(r, 5000, 10000)
	offset := parseOffsetQuery(r)

	orders, err := s.Admin.ListOrdersAdmin(r.Context(), admin.ListOrdersAdminInput{
		Status:      status,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	filename := fmt.Sprintf("sip-happens-orders-%s.csv", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if err := admin.WriteOrdersCSV(w, orders); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "export failed")
		return
	}
}
