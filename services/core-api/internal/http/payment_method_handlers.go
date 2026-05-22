package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

const (
	defaultPaymentMethodSuccessURL = "https://sip-happens.local/payment-methods/success"
	defaultPaymentMethodCancelURL  = "https://sip-happens.local/payment-methods/cancel"
)

type collectPaymentMethodsRequest struct {
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

func (s *Server) handlePaymentMethodsCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Customers == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if s.Stripe == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "unavailable", "stripe not configured")
		return
	}

	var req collectPaymentMethodsRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	successURL := strings.TrimSpace(req.SuccessURL)
	cancelURL := strings.TrimSpace(req.CancelURL)
	if successURL == "" {
		successURL = defaultPaymentMethodSuccessURL
	}
	if cancelURL == "" {
		cancelURL = defaultPaymentMethodCancelURL
	}

	stripeCustID, err := s.Customers.StripeCustomerID(r.Context(), customerID)
	if err != nil && err != domain.ErrNotFound {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if stripeCustID == "" {
		cust, err := s.Customers.GetByID(r.Context(), customerID)
		if err != nil {
			if err == domain.ErrNotFound {
				writeAPIError(w, r, http.StatusNotFound, "not_found", "customer not found")
				return
			}
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		name := strings.TrimSpace(cust.FullName)
		email := strings.TrimSpace(cust.Email)
		out, err := s.Stripe.CreateCustomer(r.Context(), stripepayments.CreateCustomerInput{
			CustomerID: customerID,
			Email:      email,
			Name:       name,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "could not create stripe customer")
			return
		}
		if err := s.Customers.SetStripeCustomerID(r.Context(), customerID, out.StripeCustomerID); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		stripeCustID = out.StripeCustomerID
	}

	collectOut, err := s.Stripe.CreatePaymentMethodCollect(r.Context(), stripepayments.CreatePaymentMethodCollectInput{
		CustomerID:       customerID,
		StripeCustomerID: stripeCustID,
		SuccessURL:       successURL,
		CancelURL:        cancelURL,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "could not start payment method collect")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"collect": map[string]string{
			"url":                 collectOut.URL,
			"checkout_session_id": collectOut.CheckoutSessionID,
		},
	})
}
