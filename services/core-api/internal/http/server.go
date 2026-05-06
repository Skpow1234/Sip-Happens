package http

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/domain"
	"sip-happens/services/core-api/internal/orders"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

type Server struct {
	Orders        orders.Service
	Catalog       catalog.Repository
	DB            *sql.DB
	Stripe        stripepayments.Service
	StripeWebhook *stripepayments.WebhookHandler
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/catalog", s.handleCatalog)
	mux.HandleFunc("/catalog/", s.handleCatalogDetail)
	mux.HandleFunc("/orders/", s.handleGetOrder)
	mux.HandleFunc("/checkout", s.handleCheckout)

	if s.StripeWebhook != nil {
		mux.Handle("/webhooks/stripe", s.StripeWebhook)
	}

	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Catalog == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if s.DB == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "COP"
	}

	list, err := s.Catalog.ListActiveProducts(r.Context(), s.DB, currency, 100)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"currency": currency,
		"items":    list,
	})
}

func (s *Server) handleCatalogDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Catalog == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if s.DB == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	sku := strings.TrimPrefix(r.URL.Path, "/catalog/")
	if sku == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "COP"
	}

	p, err := s.Catalog.GetProductBySKU(r.Context(), s.DB, sku)
	if err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, "error", status)
		return
	}
	price, err := s.Catalog.GetCurrentPrice(r.Context(), s.DB, p.ID, currency)
	if err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, "error", status)
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"product": p,
		"price":   price,
	})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	o, items, err := s.Orders.GetOrder(r.Context(), domain.OrderID(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, "error", status)
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order": o,
		"items": items,
	})
}

type checkoutRequest struct {
	CustomerID string `json:"customer_id"`
	Currency   string `json:"currency"`

	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	AddressLine1 string `json:"address_line1"`
	Phone        string `json:"phone"`
	LegalID      string `json:"legal_id"`
	FullName     string `json:"full_name"`

	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`

	Items []struct {
		ProductID string `json:"product_id"`
		Qty       int    `json:"qty"`
	} `json:"items"`
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	in := orders.CreatePendingOrderInput{
		CustomerID: domain.CustomerID(req.CustomerID),
		Currency:   req.Currency,

		City:         req.City,
		Neighborhood: req.Neighborhood,
		AddressLine1: req.AddressLine1,
		Phone:        req.Phone,
		LegalID:      req.LegalID,
		FullName:     req.FullName,
	}
	for _, it := range req.Items {
		in.Items = append(in.Items, orders.CreatePendingOrderItem{
			ProductID: domain.ProductID(it.ProductID),
			Qty:       it.Qty,
		})
	}

	o, items, err := s.Orders.CreatePendingOrder(r.Context(), in)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case domain.ErrInvalidInput:
			status = http.StatusBadRequest
		case domain.ErrForbidden:
			status = http.StatusForbidden
		case domain.ErrConflict:
			status = http.StatusConflict
		}
		http.Error(w, "error", status)
		return
	}

	var stripeOut *stripepayments.CreateCheckoutOutput
	if s.Stripe != nil {
		exp := time.Now().Add(15 * time.Minute)
		if o.ExpiresAt != nil {
			exp = *o.ExpiresAt
		}
		stripeOut, err = s.Stripe.CreateCheckout(r.Context(), stripepayments.CreateCheckoutInput{
			OrderID:    o.ID,
			Currency:   o.Currency,
			Total:      o.TotalMinorUnits,
			ExpiresAt:  exp,
			SuccessURL: req.SuccessURL,
			CancelURL:  req.CancelURL,
		})
		if err != nil {
			http.Error(w, "stripe error", http.StatusBadGateway)
			return
		}

		_ = s.Orders.SetStripeRefs(r.Context(), o.ID, stripeOut.CheckoutSessionID, stripeOut.PaymentIntentID)
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order": o,
		"items": items,
		"stripe": stripeOut,
	})
}

