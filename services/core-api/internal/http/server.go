package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/orders"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	Orders        orders.Service
	Catalog       catalog.Repository
	Admin         admin.Service
	AdminToken    string
	DB            *sql.DB
	Stripe        stripepayments.Service
	StripeWebhook *stripepayments.WebhookHandler

	CoreAPIHMACSecret string
}

func (s *Server) Handler() http.Handler {
	metrics := NewMetrics()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/catalog", s.handleCatalog)
	mux.HandleFunc("/catalog/", s.handleCatalogDetail)
	mux.Handle("/orders/", s.requireSigned(http.HandlerFunc(s.handleGetOrder)))
	mux.Handle("/checkout", s.requireSigned(http.HandlerFunc(s.handleCheckout)))
	mux.HandleFunc("/admin/products", s.handleAdminProducts)
	mux.HandleFunc("/admin/products/", s.handleAdminProductDetail)
	mux.HandleFunc("/admin/prices", s.handleAdminPrices)
	mux.HandleFunc("/admin/inventory", s.handleAdminInventory)
	mux.HandleFunc("/admin/audit-events", s.handleAdminAuditEvents)

	if s.StripeWebhook != nil {
		mux.Handle("/webhooks/stripe", s.StripeWebhook)
	}

	h := http.Handler(mux)
	h = WithRequestID(h)
	h = AccessLog(metrics)(h)
	return h
}

func (s *Server) requireSigned(next http.Handler) http.Handler {
	// For local prototyping, if no secret is configured we accept unsigned requests.
	if strings.TrimSpace(s.CoreAPIHMACSecret) == "" {
		return next
	}
	m := &HMACMiddleware{Secret: []byte(s.CoreAPIHMACSecret), Tolerance: 5 * time.Minute}
	return m.Wrap(next)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.PingContext(ctx); err != nil {
		http.Error(w, "db not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.AdminToken == "" {
		http.Error(w, "admin not configured", http.StatusNotFound)
		return false
	}
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) || strings.TrimPrefix(h, prefix) != s.AdminToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) handleAdminProducts(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		currency := r.URL.Query().Get("currency")
		if currency == "" {
			currency = "COP"
		}
		limit := parseIntDefault(r.URL.Query().Get("limit"), 200)
		offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
		q := r.URL.Query().Get("q")
		var isActivePtr *bool
		if v := strings.TrimSpace(r.URL.Query().Get("active")); v != "" {
			b := v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
			isActivePtr = &b
		}

		list, err := s.Admin.ListProducts(r.Context(), admin.ListProductsInput{
			Currency: currency,
			Limit:    limit,
			Offset:   offset,
			Query:    q,
			IsActive: isActivePtr,
		})
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": list, "limit": limit, "offset": offset})
	case http.MethodPost:
		var req admin.UpsertProductInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		id, err := s.Admin.UpsertProduct(r.Context(), "admin", req)
		if err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusBadRequest
			}
			http.Error(w, "error", status)
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"product_id": id})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminProductDetail(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/admin/products/")
	if id == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		reason := r.URL.Query().Get("reason")
		err := s.Admin.DeactivateProduct(r.Context(), "admin", domain.ProductID(id), reason)
		if err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusBadRequest
			}
			if err == domain.ErrNotFound {
				status = http.StatusNotFound
			}
			http.Error(w, "error", status)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminPrices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req admin.SetPriceInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "COP"
	}
	if err := s.Admin.SetPrice(r.Context(), "admin", req); err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrInvalidInput {
			status = http.StatusBadRequest
		}
		http.Error(w, "error", status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminInventory(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req admin.UpsertInventoryInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.Admin.UpsertInventory(r.Context(), "admin", req); err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrInvalidInput {
			status = http.StatusBadRequest
		}
		http.Error(w, "error", status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminAuditEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 200)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	aggType := r.URL.Query().Get("aggregate_type")
	aggID := r.URL.Query().Get("aggregate_id")

	evts, err := s.Admin.ListAuditEvents(r.Context(), admin.ListAuditEventsInput{
		AggregateType: aggType,
		AggregateID:   aggID,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": evts, "limit": limit, "offset": offset})
}

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	return n
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

	customerID := domain.CustomerID(req.CustomerID)
	if customerID == "" {
		fp := r.Header.Get("X-SSH-Fingerprint")
		if fp != "" && s.DB != nil {
			id, err := s.getOrCreateCustomerIDByFingerprint(r.Context(), fp)
			if err == nil {
				customerID = id
			}
		}
	}
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	in := orders.CreatePendingOrderInput{
		CustomerID: customerID,
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

func (s *Server) getOrCreateCustomerIDByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error) {
	if fp == "" {
		return "", domain.ErrInvalidInput
	}
	// Lookup by fingerprint first.
	row := s.DB.QueryRowContext(ctx, `select id from customers where ssh_fingerprint=$1`, fp)
	var id string
	if err := row.Scan(&id); err == nil {
		return domain.CustomerID(id), nil
	}

	// Insert.
	uuid, err := ids.UUIDv4()
	if err != nil {
		return "", err
	}
	newID := uuid
	_, err = s.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,now(),now())
on conflict (ssh_fingerprint) do nothing
`, newID, fp)
	if err != nil {
		return "", err
	}
	// Re-read to return canonical id.
	row2 := s.DB.QueryRowContext(ctx, `select id from customers where ssh_fingerprint=$1`, fp)
	if err := row2.Scan(&id); err != nil {
		return "", err
	}
	return domain.CustomerID(id), nil
}

