package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
)

// handleAdminProductPriceHistory returns the full price ledger for a product
// (every set/end of every currency) joined with the matching admin audit
// rows. Roadmap items 132–133.
//
// Auth: AdminRoleFinance (read-only). Path:
//
//	GET /admin/products/{id}/price-history
//
// Response shape:
//
//	{
//	  "product_id": "prod-…",
//	  "entries": [
//	    { "currency": "COP", "amount_minor_units": 1100, "valid_from": "…",
//	      "valid_through": null, "set_by": "admin", "set_at": "…" },
//	    { "currency": "COP", "amount_minor_units": 1000, "valid_from": "…",
//	      "valid_through": "…", "set_by": "admin", "set_at": "…",
//	      "ended_by": "admin", "ended_reason": "promo_end" },
//	    …
//	  ]
//	}
//
// Entries are ordered newest-valid_from first, with currencies interleaved
// by time; the admin console panel groups them by currency client-side.
// Returns an empty `entries: []` rather than 404 when the product exists
// but has never had a price (an unusual but legal state in tests).
func (s *Server) handleAdminProductPriceHistory(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing product id")
		return
	}
	productID := domain.ProductID(id)

	entries, err := s.Admin.ListPriceHistory(r.Context(), productID)
	if err != nil {
		if err == domain.ErrInvalidInput {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid product id")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"product_id": string(productID),
		"entries":    entries,
	})
}
