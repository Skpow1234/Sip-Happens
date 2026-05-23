package http

import (
	"encoding/json"
	"net/http"
	"strings"

	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

type cartPaymentMethodPutRequest struct {
	PaymentMethodID string `json:"payment_method_id"`
}

func (s *Server) handleCartPaymentMethod(w http.ResponseWriter, r *http.Request) {
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

	var req cartPaymentMethodPutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	pmID := strings.TrimSpace(req.PaymentMethodID)
	if pmID != "" {
		if s.Stripe == nil {
			writeAPIError(w, r, http.StatusServiceUnavailable, "unavailable", "stripe not configured")
			return
		}
		stripeCustID, err := s.stripeCustomerIDOrEmpty(r.Context(), w, r, customerID)
		if err != nil {
			return
		}
		if stripeCustID == "" {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "payment method not found")
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

	if err := s.Customers.SetCartPaymentMethodID(r.Context(), customerID, pmID); err != nil {
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
