package http

import (
	"context"
	"encoding/json"
	"errors"
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

func (s *Server) handlePaymentMethods(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handlePaymentMethodsList(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePaymentMethodByID(w http.ResponseWriter, r *http.Request) {
	ref := strings.TrimPrefix(r.URL.Path, "/payment-methods/")
	if ref == "" || ref == "collect" {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handlePaymentMethodGet(w, r, ref)
	case http.MethodDelete:
		s.handlePaymentMethodDelete(w, r, ref)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePaymentMethodsList(w http.ResponseWriter, r *http.Request) {
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

	stripeCustID, err := s.Customers.StripeCustomerID(r.Context(), customerID)
	if err != nil && err != domain.ErrNotFound {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if stripeCustID == "" {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"payment_methods": []stripepayments.PaymentMethodCard{}})
		return
	}

	cards, err := s.Stripe.ListPaymentMethods(r.Context(), stripepayments.ListPaymentMethodsInput{
		StripeCustomerID: stripeCustID,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "could not list payment methods")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"payment_methods": cards})
}

func (s *Server) handlePaymentMethodGet(w http.ResponseWriter, r *http.Request, paymentMethodID string) {
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

	stripeCustID, err := s.stripeCustomerIDOrEmpty(r.Context(), w, r, customerID)
	if err != nil || stripeCustID == "" {
		if err == nil {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "payment method not found")
		}
		return
	}

	card, err := s.Stripe.GetPaymentMethod(r.Context(), stripepayments.GetPaymentMethodInput{
		StripeCustomerID: stripeCustID,
		PaymentMethodID:  paymentMethodID,
	})
	if err != nil {
		s.writePaymentMethodStripeError(w, r, err)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"payment_method": card})
}

func (s *Server) handlePaymentMethodDelete(w http.ResponseWriter, r *http.Request, paymentMethodID string) {
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

	stripeCustID, err := s.stripeCustomerIDOrEmpty(r.Context(), w, r, customerID)
	if err != nil || stripeCustID == "" {
		if err == nil {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "payment method not found")
		}
		return
	}

	err = s.Stripe.DetachPaymentMethod(r.Context(), stripepayments.DetachPaymentMethodInput{
		StripeCustomerID: stripeCustID,
		PaymentMethodID:  paymentMethodID,
	})
	if err != nil {
		s.writePaymentMethodStripeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

	stripeCustID, err := s.ensureStripeCustomerID(r.Context(), w, r, customerID)
	if err != nil {
		return
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

func (s *Server) stripeCustomerIDOrEmpty(ctx context.Context, w http.ResponseWriter, r *http.Request, customerID domain.CustomerID) (string, error) {
	stripeCustID, err := s.Customers.StripeCustomerID(ctx, customerID)
	if err != nil && err != domain.ErrNotFound {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return "", err
	}
	return stripeCustID, nil
}

// ensureStripeCustomerID returns the Stripe customer id, creating and persisting one when missing.
func (s *Server) ensureStripeCustomerID(ctx context.Context, w http.ResponseWriter, r *http.Request, customerID domain.CustomerID) (string, error) {
	stripeCustID, err := s.stripeCustomerIDOrEmpty(ctx, w, r, customerID)
	if err != nil {
		return "", err
	}
	if stripeCustID != "" {
		return stripeCustID, nil
	}
	cust, err := s.Customers.GetByID(ctx, customerID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "customer not found")
			return "", err
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return "", err
	}
	out, err := s.Stripe.CreateCustomer(ctx, stripepayments.CreateCustomerInput{
		CustomerID: customerID,
		Email:      strings.TrimSpace(cust.Email),
		Name:       strings.TrimSpace(cust.FullName),
	})
	if err != nil {
		writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "could not create stripe customer")
		return "", err
	}
	if err := s.Customers.SetStripeCustomerID(ctx, customerID, out.StripeCustomerID); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return "", err
	}
	return out.StripeCustomerID, nil
}

func (s *Server) writePaymentMethodStripeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, stripepayments.ErrPaymentMethodNotFound) || errors.Is(err, stripepayments.ErrPaymentMethodNotOwned) {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "payment method not found")
		return
	}
	writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "stripe payment method error")
}
