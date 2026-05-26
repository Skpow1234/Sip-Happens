package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	orderpkg "sip-happens/services/core-api/internal/orders"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

type orderCreateRequest struct {
	Currency        string `json:"currency"`
	AddressID       string `json:"address_id"`
	City            string `json:"city"`
	Neighborhood    string `json:"neighborhood"`
	AddressLine1    string `json:"address_line1"`
	Phone           string `json:"phone"`
	LegalID         string `json:"legal_id"`
	FullName        string `json:"full_name"`
	BirthDate       string `json:"birth_date"`
	PrivacyConsent  bool   `json:"privacy_consent"`
	SuccessURL      string `json:"success_url"`
	CancelURL       string `json:"cancel_url"`
	PaymentMethodID string `json:"payment_method_id"`
	Items           []struct {
		ProductID string `json:"product_id"`
		Qty       int    `json:"qty"`
	} `json:"items"`
}

func (s *Server) handleOrderCreate(w http.ResponseWriter, r *http.Request) {
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

	var req orderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	items := make([]orderpkg.CreatePendingOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Qty <= 0 {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid item qty")
			return
		}
		ref := strings.TrimSpace(it.ProductID)
		if ref == "" {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid item")
			return
		}
		items = append(items, orderpkg.CreatePendingOrderItem{
			ProductID: domain.ProductID(ref),
			Qty:       it.Qty,
		})
	}

	delivery, err := s.resolveOrderDelivery(r.Context(), customerID, deliveryFieldOverrides{
		AddressID:    req.AddressID,
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
	if pmID := strings.TrimSpace(req.PaymentMethodID); pmID != "" {
		if s.Stripe == nil {
			writeAPIError(w, r, http.StatusServiceUnavailable, "unavailable", "stripe not configured")
			return
		}
		stripeCustID, err = s.stripeCustomerIDOrEmpty(r.Context(), w, r, customerID)
		if err != nil {
			return
		}
		if stripeCustID == "" {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "payment method requires saved cards")
			return
		}
		if _, err := s.Stripe.GetPaymentMethod(r.Context(), stripepayments.GetPaymentMethodInput{
			StripeCustomerID: stripeCustID,
			PaymentMethodID:  pmID,
		}); err != nil {
			s.writePaymentMethodStripeError(w, r, err)
			return
		}
	}

	s.placeOrderAndRespond(w, r, placeOrderParams{
		CustomerID:           customerID,
		Currency:             req.Currency,
		Delivery:             delivery,
		BirthDate:            req.BirthDate,
		PrivacyConsent:       req.PrivacyConsent,
		SuccessURL:           req.SuccessURL,
		CancelURL:            req.CancelURL,
		Items:                items,
		AgeVerifyMethod:      "order_create",
		ClearCartItems:       false,
		ClearCartSelections:  false,
		StripeCustomerIDHint: stripeCustID,
	})
}
