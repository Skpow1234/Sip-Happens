package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/addresses"
)

func (s *Server) handleAddresses(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAddressesList(w, r)
	case http.MethodPost:
		s.handleAddressesCreate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAddressByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAddressGet(w, r)
	case http.MethodDelete:
		s.handleAddressDelete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAddressesList(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Addresses == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	list, err := s.Addresses.ListByCustomer(r.Context(), s.DB, customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	items := make([]addresses.AddressView, 0, len(list))
	for _, a := range list {
		items = append(items, addresses.AddressViewFromDomain(a))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"addresses": items})
}

type createAddressRequest struct {
	Label        string `json:"label"`
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	LegalID      string `json:"legal_id"`
	AddressLine1 string `json:"address_line1"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	IsDefault    *bool  `json:"is_default"`
}

func (s *Server) handleAddressesCreate(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Addresses == nil || s.DB == nil || s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	var req createAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	in := addresses.NewAddressInput{
		Label:        req.Label,
		FullName:     req.FullName,
		Phone:        req.Phone,
		LegalID:      req.LegalID,
		AddressLine1: req.AddressLine1,
		Neighborhood: req.Neighborhood,
		City:         req.City,
	}
	if c, err := s.Customers.GetByID(r.Context(), customerID); err == nil && c != nil {
		if strings.TrimSpace(in.FullName) == "" {
			in.FullName = c.FullName
		}
		if strings.TrimSpace(in.Phone) == "" {
			in.Phone = c.Phone
		}
		if strings.TrimSpace(in.LegalID) == "" {
			in.LegalID = c.LegalID
		}
	}
	canonCity, cityKey, err := addresses.ValidateNewAddress(in)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid address (city must be Bogotá, Medellín, or Cali)")
		return
	}

	existing, err := s.Addresses.ListByCustomer(r.Context(), s.DB, customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	isDefault := len(existing) == 0
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	addrUUID, err := ids.UUIDv4()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	now := time.Now()
	addr := domain.CustomerAddress{
		ID:             domain.AddressID(addrUUID),
		CustomerID:     customerID,
		Label:          strings.TrimSpace(in.Label),
		FullName:       strings.TrimSpace(in.FullName),
		Phone:          strings.TrimSpace(in.Phone),
		LegalID:        strings.TrimSpace(in.LegalID),
		AddressLine1:   strings.TrimSpace(in.AddressLine1),
		Neighborhood:   strings.TrimSpace(in.Neighborhood),
		City:           canonCity,
		CityNormalized: cityKey,
		IsDefault:      isDefault,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.Addresses.Create(r.Context(), s.DB, addr); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"address": addresses.AddressViewFromDomain(addr),
	})
}

func (s *Server) handleAddressGet(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/addresses/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing address id")
		return
	}
	if s.Addresses == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	a, err := addresses.ResolveAddress(r.Context(), s.Addresses, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "address not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"address": addresses.AddressViewFromDomain(*a),
	})
}

func (s *Server) handleAddressDelete(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/addresses/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing address id")
		return
	}
	if s.Addresses == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	a, err := addresses.ResolveAddress(r.Context(), s.Addresses, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "address not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if err := s.Addresses.Delete(r.Context(), s.DB, customerID, a.ID); err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "address not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) customerIDFromRequest(w http.ResponseWriter, r *http.Request) (domain.CustomerID, bool) {
	if id, ok := patCustomerIDFromContext(r.Context()); ok {
		return id, true
	}
	if s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return "", false
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return "", false
	}
	customerID, err := s.Customers.GetOrCreateByFingerprint(r.Context(), fp)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return "", false
	}
	return customerID, true
}
