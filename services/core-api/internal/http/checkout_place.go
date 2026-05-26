package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/obslog"
	"sip-happens/pkg/shipping"
	"sip-happens/services/core-api/internal/customers"
	orderpkg "sip-happens/services/core-api/internal/orders"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

const (
	defaultCheckoutSuccessURL = "https://sip-happens.local/success"
	defaultCheckoutCancelURL  = "https://sip-happens.local/cancel"
)

type checkoutRequest struct {
	CustomerID string `json:"customer_id"`
	Currency   string `json:"currency"`

	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	AddressLine1 string `json:"address_line1"`
	Phone        string `json:"phone"`
	LegalID      string `json:"legal_id"`
	FullName     string `json:"full_name"`
	BirthDate    string `json:"birth_date"`
	// PrivacyConsent must be true to place an order (records customer_consents).
	PrivacyConsent bool `json:"privacy_consent"`

	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`

	Items []struct {
		ProductID string `json:"product_id"`
		Qty       int    `json:"qty"`
	} `json:"items"`
}

type deliverySnapshot struct {
	City         string
	Neighborhood string
	AddressLine1 string
	Phone        string
	LegalID      string
	FullName     string
}

func (d deliverySnapshot) complete() bool {
	return strings.TrimSpace(d.City) != "" &&
		strings.TrimSpace(d.Neighborhood) != "" &&
		strings.TrimSpace(d.AddressLine1) != "" &&
		strings.TrimSpace(d.Phone) != "" &&
		strings.TrimSpace(d.LegalID) != "" &&
		strings.TrimSpace(d.FullName) != ""
}

type placeOrderParams struct {
	CustomerID           domain.CustomerID
	Currency             string
	Delivery             deliverySnapshot
	BirthDate            string
	PrivacyConsent       bool
	SuccessURL           string
	CancelURL            string
	Items                []orderpkg.CreatePendingOrderItem
	AgeVerifyMethod      string
	ClearCartItems       bool
	ClearCartSelections  bool
	StripeCustomerIDHint string
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Orders == nil || s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	customerID := domain.CustomerID(req.CustomerID)
	if customerID == "" {
		fp := r.Header.Get("X-SSH-Fingerprint")
		if fp != "" && s.Customers != nil {
			id, err := s.Customers.GetOrCreateByFingerprint(r.Context(), fp)
			if err == nil {
				customerID = id
			}
		}
	}
	if customerID == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing customer_id (or X-SSH-Fingerprint)")
		return
	}

	items := make([]orderpkg.CreatePendingOrderItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, orderpkg.CreatePendingOrderItem{
			ProductID: domain.ProductID(it.ProductID),
			Qty:       it.Qty,
		})
	}

	s.placeOrderAndRespond(w, r, placeOrderParams{
		CustomerID: customerID,
		Currency:   req.Currency,
		Delivery: deliverySnapshot{
			City:         req.City,
			Neighborhood: req.Neighborhood,
			AddressLine1: req.AddressLine1,
			Phone:        req.Phone,
			LegalID:      req.LegalID,
			FullName:     req.FullName,
		},
		BirthDate:           req.BirthDate,
		PrivacyConsent:      req.PrivacyConsent,
		SuccessURL:          req.SuccessURL,
		CancelURL:           req.CancelURL,
		Items:               items,
		AgeVerifyMethod:     "checkout",
		ClearCartItems:      true,
		ClearCartSelections: false,
	})
}

func (s *Server) placeOrderAndRespond(w http.ResponseWriter, r *http.Request, in placeOrderParams) {
	if !in.PrivacyConsent {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "privacy_consent required")
		return
	}
	if len(in.Items) == 0 {
		msg := "items required"
		if in.AgeVerifyMethod == "cart_convert" {
			msg = "cart is empty"
		}
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", msg)
		return
	}
	if !in.Delivery.complete() {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "incomplete delivery details")
		return
	}
	if strings.TrimSpace(in.Currency) == "" {
		in.Currency = "COP"
	}
	if strings.TrimSpace(in.SuccessURL) == "" {
		in.SuccessURL = defaultCheckoutSuccessURL
	}
	if strings.TrimSpace(in.CancelURL) == "" {
		in.CancelURL = defaultCheckoutCancelURL
	}
	if strings.TrimSpace(in.AgeVerifyMethod) == "" {
		in.AgeVerifyMethod = "checkout"
	}

	canonCity, err := shipping.CanonicalCity(in.Delivery.City)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}
	in.Delivery.City = canonCity

	cityKey := shipping.CityKey(canonCity)
	shipMinor, err := shipping.LookupAmountMinor(r.Context(), s.DB, cityKey, in.Currency)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	if err := s.Customers.SaveCheckoutProfile(r.Context(), in.CustomerID, customers.CheckoutProfile{
		City:         in.Delivery.City,
		Neighborhood: in.Delivery.Neighborhood,
		AddressLine1: in.Delivery.AddressLine1,
		Phone:        in.Delivery.Phone,
		LegalID:      in.Delivery.LegalID,
		FullName:     in.Delivery.FullName,
		BirthDate:    in.BirthDate,
	}); err != nil {
		if err == domain.ErrInvalidInput {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid birth_date or age")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = s.Customers.RecordConsent(r.Context(), in.CustomerID, "privacy_policy")

	orderIn := orderpkg.CreatePendingOrderInput{
		CustomerID:         in.CustomerID,
		Currency:           in.Currency,
		City:               in.Delivery.City,
		Neighborhood:       in.Delivery.Neighborhood,
		AddressLine1:       in.Delivery.AddressLine1,
		Phone:              in.Delivery.Phone,
		LegalID:            in.Delivery.LegalID,
		FullName:           in.Delivery.FullName,
		ShippingMinorUnits: shipMinor,
		Items:              in.Items,
	}

	checkoutCtx := obslog.WithCustomerID(r.Context(), string(in.CustomerID))
	o, items, err := s.Orders.CreatePendingOrder(checkoutCtx, orderIn)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		switch err {
		case domain.ErrInvalidInput:
			status = http.StatusUnprocessableEntity
			code = "invalid_input"
		case domain.ErrForbidden:
			status = http.StatusForbidden
			code = "forbidden"
		case domain.ErrConflict:
			status = http.StatusConflict
			code = "conflict"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}

	birthForAge := strings.TrimSpace(in.BirthDate)
	if birthForAge == "" && s.Customers != nil {
		if cust, err := s.Customers.GetByID(r.Context(), in.CustomerID); err == nil && cust != nil && !cust.BirthDate.IsZero() {
			birthForAge = cust.BirthDate.Format("2006-01-02")
		}
	}
	if err := s.Customers.RecordAgeVerification(r.Context(), in.CustomerID, o.ID, birthForAge, in.AgeVerifyMethod); err != nil {
		if err == domain.ErrInvalidInput {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "age verification required")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if s.Admin != nil {
		actor := strings.TrimSpace(r.Header.Get("X-SSH-Fingerprint"))
		if actor == "" {
			actor = "customer:" + string(in.CustomerID)
		}
		payload, _ := json.Marshal(map[string]string{
			"method":      in.AgeVerifyMethod,
			"customer_id": string(in.CustomerID),
		})
		_ = s.Admin.RecordAudit(r.Context(), actor, "order", string(o.ID), "age.verified", string(payload))
	}

	stripeCustID := strings.TrimSpace(in.StripeCustomerIDHint)
	if stripeCustID == "" && s.Customers != nil {
		if id, err := s.Customers.StripeCustomerID(r.Context(), in.CustomerID); err == nil {
			stripeCustID = id
		}
	}

	var stripeOut *stripepayments.CreateCheckoutOutput
	if s.Stripe != nil {
		exp := time.Now().Add(15 * time.Minute)
		if o.ExpiresAt != nil {
			exp = *o.ExpiresAt
		}
		stripeOut, err = s.Stripe.CreateCheckout(r.Context(), stripepayments.CreateCheckoutInput{
			OrderID:            o.ID,
			Currency:           o.Currency,
			SubtotalMinorUnits: o.SubtotalMinorUnits,
			ShippingMinorUnits: o.ShippingMinorUnits,
			TotalMinorUnits:    o.TotalMinorUnits,
			ExpiresAt:          exp,
			SuccessURL:         in.SuccessURL,
			CancelURL:          in.CancelURL,
			StripeCustomerID:   stripeCustID,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusBadGateway, "bad_gateway", "stripe error")
			return
		}

		_ = s.Orders.SetStripeRefs(r.Context(), o.ID, stripeOut.CheckoutSessionID, stripeOut.PaymentIntentID)
		if s.Admin != nil {
			actor := strings.TrimSpace(r.Header.Get("X-SSH-Fingerprint"))
			if actor == "" {
				actor = "customer:" + string(in.CustomerID)
			}
			linkPayload, _ := json.Marshal(map[string]string{
				"checkout_session_id": stripeOut.CheckoutSessionID,
				"payment_intent_id":   stripeOut.PaymentIntentID,
			})
			_ = s.Admin.RecordAudit(r.Context(), actor, "order", string(o.ID), "payment.link_created", string(linkPayload))
		}
		if s.Payments != nil {
			_ = s.Payments.RecordStatus(r.Context(), s.DB, domain.PaymentStatusHistoryEntry{
				OrderID:     o.ID,
				Provider:    domain.PaymentProviderStripe,
				ProviderRef: stripeOut.CheckoutSessionID,
				Status:      domain.PaymentStatusCreated,
				Source:      in.AgeVerifyMethod,
				Detail:      `{"event":"stripe_checkout_created"}`,
			})
			if stripeOut.PaymentIntentID != "" {
				_ = s.Payments.RecordStatus(r.Context(), s.DB, domain.PaymentStatusHistoryEntry{
					OrderID:     o.ID,
					Provider:    domain.PaymentProviderStripe,
					ProviderRef: stripeOut.PaymentIntentID,
					Status:      domain.PaymentStatusPending,
					Source:      in.AgeVerifyMethod,
					Detail:      `{"event":"stripe_payment_intent_linked"}`,
				})
			}
		}
		if stripeOut.URL != "" {
			s.maybeSendPaymentLinkEmail(o, in.Delivery.FullName, stripeOut.URL)
		}
	}

	if in.ClearCartSelections {
		_ = s.Customers.ClearCart(r.Context(), in.CustomerID)
	} else if in.ClearCartItems {
		_ = s.Customers.SaveCart(r.Context(), in.CustomerID, nil)
	}

	itemViews := make([]orderpkg.OrderItemView, 0, len(items))
	for _, it := range items {
		itemViews = append(itemViews, orderpkg.OrderItemViewFromDomain(it))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order":  orderpkg.OrderDetailViewFromDomain(*o),
		"items":  itemViews,
		"stripe": stripeOut,
	})
}

func pickDeliveryField(override, saved, profile string) string {
	if s := strings.TrimSpace(override); s != "" {
		return s
	}
	if s := strings.TrimSpace(saved); s != "" {
		return s
	}
	return strings.TrimSpace(profile)
}

func (s *Server) customerDeliveryProfile(ctx context.Context, customerID domain.CustomerID) deliverySnapshot {
	var d deliverySnapshot
	if s.Customers == nil {
		return d
	}
	cust, err := s.Customers.GetByID(ctx, customerID)
	if err != nil || cust == nil {
		return d
	}
	return deliverySnapshot{
		City:         cust.City,
		Neighborhood: cust.Neighborhood,
		AddressLine1: cust.AddressLine1,
		Phone:        cust.Phone,
		LegalID:      cust.LegalID,
		FullName:     cust.FullName,
	}
}
