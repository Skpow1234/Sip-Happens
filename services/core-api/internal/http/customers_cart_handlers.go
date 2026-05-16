package http

import (
	"encoding/json"
	"net/http"
	"time"

	"sip-happens/pkg/age"
	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/customers"
)

type patchCustomerRequest struct {
	City         *string `json:"city"`
	Neighborhood *string `json:"neighborhood"`
	AddressLine1 *string `json:"address_line1"`
	Phone        *string `json:"phone"`
	LegalID      *string `json:"legal_id"`
	FullName     *string `json:"full_name"`
	BirthDate    *string `json:"birth_date"`
}

func (s *Server) handleCustomerMe(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleCustomerMeGet(w, r)
	case http.MethodPatch:
		s.handleCustomerMePatch(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCustomerMeGet(w http.ResponseWriter, r *http.Request) {
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
	c, err := s.Customers.GetByID(r.Context(), customerID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "customer not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"customer": customerJSON(c)})
}

func (s *Server) handleCustomerMePatch(w http.ResponseWriter, r *http.Request) {
	if s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	var req patchCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	customerID, err := s.Customers.GetOrCreateByFingerprint(r.Context(), fp)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	c, err := s.Customers.GetByID(r.Context(), customerID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "customer not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if req.City != nil {
		c.City = *req.City
	}
	if req.Neighborhood != nil {
		c.Neighborhood = *req.Neighborhood
	}
	if req.AddressLine1 != nil {
		c.AddressLine1 = *req.AddressLine1
	}
	if req.Phone != nil {
		c.Phone = *req.Phone
	}
	if req.LegalID != nil {
		c.LegalID = *req.LegalID
	}
	if req.FullName != nil {
		c.FullName = *req.FullName
	}
	if req.BirthDate != nil {
		bd, err := customers.ParseBirthDate(*req.BirthDate)
		if err != nil {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid birth_date (use YYYY-MM-DD)")
			return
		}
		if !age.IsAdult(bd, time.Now()) {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "must be 18 or older")
			return
		}
		c.BirthDate = bd
	}
	if err := s.Customers.Update(r.Context(), c); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"customer": customerJSON(c)})
}

func customerJSON(c *domain.Customer) map[string]any {
	out := map[string]any{
		"ID":           c.ID,
		"City":         c.City,
		"Neighborhood": c.Neighborhood,
		"AddressLine1": c.AddressLine1,
		"Phone":        c.Phone,
		"LegalID":      c.LegalID,
		"FullName":     c.FullName,
	}
	if !c.BirthDate.IsZero() {
		out["BirthDate"] = c.BirthDate.Format("2006-01-02")
	}
	return out
}

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
