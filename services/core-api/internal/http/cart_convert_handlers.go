package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/orders"
)

type cartConvertRequest struct {
	BirthDate      string `json:"birth_date"`
	PrivacyConsent bool   `json:"privacy_consent"`
	SuccessURL     string `json:"success_url"`
	CancelURL      string `json:"cancel_url"`

	// Optional delivery overrides when cart has no saved address_id.
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	AddressLine1 string `json:"address_line1"`
	Phone        string `json:"phone"`
	LegalID      string `json:"legal_id"`
	FullName     string `json:"full_name"`
}

func (s *Server) handleCartConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Orders == nil || s.Customers == nil {
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

	var req cartConvertRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
	}

	stored, err := s.Customers.LoadCart(r.Context(), customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	items := cartLinesToOrderItems(stored)
	if len(items) == 0 {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "cart is empty")
		return
	}

	addrID, err := s.Customers.LoadCartAddressID(r.Context(), customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	delivery, err := s.resolveOrderDelivery(r.Context(), customerID, deliveryFieldOverrides{
		AddressID:    string(addrID),
		City:         req.City,
		Neighborhood: req.Neighborhood,
		AddressLine1: req.AddressLine1,
		Phone:        req.Phone,
		LegalID:      req.LegalID,
		FullName:     req.FullName,
	})
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "unknown address")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	stripeCustID := ""
	if pmID, err := s.Customers.LoadCartPaymentMethodID(r.Context(), customerID); err == nil && pmID != "" {
		if id, err := s.Customers.StripeCustomerID(r.Context(), customerID); err == nil {
			stripeCustID = id
		}
	}

	s.placeOrderAndRespond(w, r, placeOrderParams{
		CustomerID:           customerID,
		Currency:             cartCurrencyFromRequest(r),
		Delivery:             delivery,
		BirthDate:            req.BirthDate,
		PrivacyConsent:       req.PrivacyConsent,
		SuccessURL:           req.SuccessURL,
		CancelURL:            req.CancelURL,
		Items:                items,
		AgeVerifyMethod:      "cart_convert",
		ClearCartItems:       true,
		ClearCartSelections:  true,
		StripeCustomerIDHint: stripeCustID,
	})
}

func cartLinesToOrderItems(stored []customers.CartLine) []orders.CreatePendingOrderItem {
	out := make([]orders.CreatePendingOrderItem, 0, len(stored))
	for _, line := range stored {
		if line.Qty <= 0 {
			continue
		}
		ref := strings.TrimSpace(line.ProductID)
		if ref == "" {
			continue
		}
		out = append(out, orders.CreatePendingOrderItem{
			ProductID: domain.ProductID(ref),
			Qty:       line.Qty,
		})
	}
	return out
}
