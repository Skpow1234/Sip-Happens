package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/addresses"
)

type cartAddressPutRequest struct {
	AddressID string `json:"address_id"`
}

func (s *Server) handleCartAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
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

	var req cartAddressPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	var internalAddr domain.AddressID
	if ref := strings.TrimSpace(req.AddressID); ref != "" {
		if s.Addresses == nil || s.DB == nil {
			writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
			return
		}
		a, err := addresses.ResolveAddress(r.Context(), s.Addresses, s.DB, customerID, ref)
		if err != nil {
			if err == domain.ErrNotFound {
				writeAPIError(w, r, http.StatusNotFound, "not_found", "unknown address")
				return
			}
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		internalAddr = a.ID
	}

	if err := s.Customers.SetCartAddressID(r.Context(), customerID, internalAddr); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	stored, err := s.Customers.LoadCart(r.Context(), customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	view, err := s.buildCartView(r, customerID, stored)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.writeCartJSON(w, view)
}
