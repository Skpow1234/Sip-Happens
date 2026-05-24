package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/catalog"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/subscriptions"
)

const (
	defaultSubscriptionSuccessURL = "https://sip-happens.local/subscriptions/success"
	defaultSubscriptionCancelURL  = "https://sip-happens.local/subscriptions/cancel"
)

func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSubscriptionsList(w, r)
	case http.MethodPost:
		s.handleSubscriptionCreate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSubscriptionGet(w, r)
	case http.MethodPut:
		s.handleSubscriptionUpdate(w, r)
	case http.MethodDelete:
		s.handleSubscriptionCancel(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSubscriptionsList(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Subscriptions == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	limit := parseLimitQuery(r, 20, 100)
	offset := parseOffsetQuery(r)
	page, err := s.Subscriptions.ListByCustomer(r.Context(), s.DB, customerID, limit, offset)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	items := make([]subscriptions.SubscriptionView, 0, len(page.Subscriptions))
	for _, sub := range page.Subscriptions {
		items = append(items, subscriptions.SubscriptionViewFromDomain(sub))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscriptions": items,
		"limit":         limit,
		"offset":        offset,
		"has_more":      page.HasMore,
	})
}

type createSubscriptionRequest struct {
	ProductID       string `json:"product_id"`
	Qty             int    `json:"qty"`
	Interval        string `json:"interval"`
	AddressID       string `json:"address_id"`
	PaymentMethodID string `json:"payment_method_id"`
	Currency        string `json:"currency"`
	SuccessURL      string `json:"success_url"`
	CancelURL       string `json:"cancel_url"`
}

func (s *Server) handleSubscriptionCreate(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Subscriptions == nil || s.Catalog == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if s.Stripe == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "unavailable", "stripe not configured")
		return
	}

	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	qty := req.Qty
	if qty <= 0 {
		qty = 1
	}
	interval := domain.SubscriptionInterval(strings.TrimSpace(req.Interval))
	if interval != domain.SubscriptionIntervalWeek && interval != domain.SubscriptionIntervalMonth {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "interval must be week or month")
		return
	}

	productRef := strings.TrimSpace(req.ProductID)
	if productRef == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "missing product_id")
		return
	}
	p, err := catalog.ResolveProduct(r.Context(), s.Catalog, s.DB, productRef)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "product not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if !p.IsActive {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "product not available")
		return
	}

	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "COP"
	}
	price, err := s.Catalog.GetCurrentPrice(r.Context(), s.DB, p.ID, currency)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "price not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	var addrID domain.AddressID
	if ref := strings.TrimSpace(req.AddressID); ref != "" {
		if s.Addresses == nil {
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
		addrID = a.ID
	}

	stripeCustID := ""
	if pmID := strings.TrimSpace(req.PaymentMethodID); pmID != "" {
		stripeCustID, err = s.stripeCustomerIDOrEmpty(r.Context(), w, r, customerID)
		if err != nil || stripeCustID == "" {
			if err == nil {
				writeAPIError(w, r, http.StatusNotFound, "not_found", "payment method not found")
			}
			return
		}
		_, err = s.Stripe.GetPaymentMethod(r.Context(), stripepayments.GetPaymentMethodInput{
			StripeCustomerID: stripeCustID,
			PaymentMethodID:  pmID,
		})
		if err != nil {
			s.writePaymentMethodStripeError(w, r, err)
			return
		}
	} else {
		stripeCustID, _ = s.stripeCustomerIDOrEmpty(r.Context(), w, r, customerID)
	}

	subUUID, err := ids.UUIDv4()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	now := time.Now()
	sub := domain.Subscription{
		ID:                   domain.SubscriptionID(subUUID),
		CustomerID:           customerID,
		ProductID:            p.ID,
		Qty:                  qty,
		Currency:             currency,
		Interval:             interval,
		UnitAmountMinorUnits: price.AmountMinorUnits,
		Status:               domain.SubscriptionStatusPending,
		AddressID:            addrID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.Subscriptions.Create(r.Context(), s.DB, sub); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	successURL := strings.TrimSpace(req.SuccessURL)
	cancelURL := strings.TrimSpace(req.CancelURL)
	if successURL == "" {
		successURL = defaultSubscriptionSuccessURL
	}
	if cancelURL == "" {
		cancelURL = defaultSubscriptionCancelURL
	}

	checkoutOut, err := s.Stripe.CreateSubscriptionCheckout(r.Context(), stripepayments.CreateSubscriptionCheckoutInput{
		SubscriptionID:       sub.ID,
		Currency:             currency,
		ProductName:          p.Name,
		Qty:                  qty,
		UnitAmountMinorUnits: price.AmountMinorUnits,
		Interval:             interval,
		SuccessURL:           successURL,
		CancelURL:            cancelURL,
		StripeCustomerID:     stripeCustID,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "could not start subscription checkout")
		return
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscription": subscriptions.SubscriptionViewFromDomain(sub),
		"stripe": map[string]string{
			"url":                 checkoutOut.URL,
			"checkout_session_id": checkoutOut.CheckoutSessionID,
		},
	})
}

func (s *Server) handleSubscriptionGet(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/subscriptions/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing subscription id")
		return
	}
	if s.Subscriptions == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	sub, err := subscriptions.ResolveSubscription(r.Context(), s.Subscriptions, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscription": subscriptions.SubscriptionViewFromDomain(*sub),
	})
}

type updateSubscriptionRequest struct {
	Qty       *int    `json:"qty"`
	AddressID *string `json:"address_id"`
}

func (s *Server) handleSubscriptionUpdate(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/subscriptions/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing subscription id")
		return
	}
	if s.Subscriptions == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	var req updateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if req.Qty == nil && req.AddressID == nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "no fields to update")
		return
	}

	in := subscriptions.UpdateInput{Qty: req.Qty}
	if req.AddressID != nil {
		refAddr := strings.TrimSpace(*req.AddressID)
		if refAddr == "" {
			in.AddressID = new(domain.AddressID)
		} else {
			if s.Addresses == nil {
				writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
				return
			}
			a, err := addresses.ResolveAddress(r.Context(), s.Addresses, s.DB, customerID, refAddr)
			if err != nil {
				if err == domain.ErrNotFound {
					writeAPIError(w, r, http.StatusNotFound, "not_found", "address not found")
					return
				}
				writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
				return
			}
			id := a.ID
			in.AddressID = &id
		}
	}

	cur, err := subscriptions.ResolveSubscription(r.Context(), s.Subscriptions, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	updated, err := s.Subscriptions.Update(r.Context(), s.DB, customerID, cur.ID, in)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		if err == domain.ErrConflict {
			writeAPIError(w, r, http.StatusConflict, "conflict", "subscription cannot be updated")
			return
		}
		if err == domain.ErrInvalidInput {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid update")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscription": subscriptions.SubscriptionViewFromDomain(*updated),
	})
}

func (s *Server) handleSubscriptionCancel(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/subscriptions/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing subscription id")
		return
	}
	if s.Subscriptions == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	cur, err := subscriptions.ResolveSubscription(r.Context(), s.Subscriptions, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	if cur.StripeSubscriptionID != "" && s.Stripe != nil {
		if err := s.Stripe.CancelSubscription(r.Context(), cur.StripeSubscriptionID); err != nil {
			writeAPIError(w, r, http.StatusBadGateway, "stripe_error", "could not cancel stripe subscription")
			return
		}
	}

	updated, err := s.Subscriptions.Cancel(r.Context(), s.DB, customerID, cur.ID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "subscription not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"subscription": subscriptions.SubscriptionViewFromDomain(*updated),
	})
}
