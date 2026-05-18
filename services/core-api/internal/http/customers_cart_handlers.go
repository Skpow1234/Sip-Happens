package http

import (
	"encoding/json"
	"net/http"

	"sip-happens/services/core-api/internal/customers"
)

func (s *Server) handleCart(w http.ResponseWriter, r *http.Request) {
	if s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.Customers.GetOrCreateByFingerprint(r.Context(), fp)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := s.Customers.LoadCart(r.Context(), customerID)
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})

	case http.MethodPut:
		var body struct {
			Items []customers.CartLine `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		if body.Items == nil {
			body.Items = []customers.CartLine{}
		}
		for _, it := range body.Items {
			if it.ProductID == "" || it.Qty <= 0 {
				writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid cart line")
				return
			}
		}
		if err := s.Customers.SaveCart(r.Context(), customerID, body.Items); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": body.Items})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
