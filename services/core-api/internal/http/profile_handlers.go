package http

import (
	"encoding/json"
	"net/http"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/customers"
)

func (s *Server) handleCustomerMe(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeCustomerProfile(w, r, "customer")
	case http.MethodPatch:
		s.patchCustomerProfile(w, r, "customer")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleProfile is a Terminal.shop alias for GET/PUT/PATCH profile.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeCustomerProfile(w, r, "profile")
	case http.MethodPut, http.MethodPatch:
		s.patchCustomerProfile(w, r, "profile")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) writeCustomerProfile(w http.ResponseWriter, r *http.Request, wrapKey string) {
	if s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	c, err := s.loadCustomerByFingerprint(r, fp)
	if err != nil {
		writeCustomerProfileError(w, r, err)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		wrapKey: customers.ProfileViewFromDomain(c),
	})
}

func (s *Server) patchCustomerProfile(w http.ResponseWriter, r *http.Request, wrapKey string) {
	if s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	var req customers.ProfilePatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	c, err := s.loadCustomerByFingerprint(r, fp)
	if err != nil {
		writeCustomerProfileError(w, r, err)
		return
	}
	if err := customers.ApplyProfilePatch(c, req, time.Now()); err != nil {
		if err == domain.ErrInvalidInput {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid profile fields (birth_date must be YYYY-MM-DD and 18+)")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if err := s.Customers.Update(r.Context(), c); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		wrapKey: customers.ProfileViewFromDomain(c),
	})
}

func (s *Server) loadCustomerByFingerprint(r *http.Request, fp string) (*domain.Customer, error) {
	customerID, err := s.Customers.GetOrCreateByFingerprint(r.Context(), fp)
	if err != nil {
		return nil, err
	}
	c, err := s.Customers.GetByID(r.Context(), customerID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func writeCustomerProfileError(w http.ResponseWriter, r *http.Request, err error) {
	if err == domain.ErrNotFound {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "customer not found")
		return
	}
	writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
}
