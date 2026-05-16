package http

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/age"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/pkg/shipping"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/orders"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	Orders            orders.Service
	Catalog           catalog.Repository
	Admin             admin.Service
	AdminToken        string
	AdminTokens       []string
	DB                *sql.DB
	Stripe            stripepayments.Service
	StripeWebhook     *stripepayments.WebhookHandler
	StripeWebhookPath string

	CoreAPIHMACSecret string
	CoreAPIHMACKeys   map[string]string

	// Namespace prefixes process-level keys (cache/ratelimit/idempotency scopes)
	// to avoid collisions if infrastructure is shared across environments.
	Namespace string

	// Feature flags for gradual rollout.
	// Names are lowercased strings (e.g. "catalog_cache").
	FeatureFlags map[string]struct{}
}

func (s *Server) Handler() http.Handler {
	metrics := NewMetrics()

	var idemStore IdempotencyStore
	if s.DB != nil {
		idemStore = &SQLIdempotencyStore{DB: s.DB}
	}
	idem := &IdempotencyMiddleware{Store: idemStore, TTL: 24 * time.Hour, Prefix: strings.TrimSpace(s.Namespace)}

	catalogCache := &SWRCache{FreshFor: 15 * time.Second, StaleFor: 45 * time.Second}

	// Best-effort rate limiting. This is not relied on for correctness and is safe
	// to be in-memory. For multi-node, limits will be per-node unless we later
	// switch to a shared backend (e.g., Redis).
	checkoutRL := (&RateLimitMiddleware{
		Limiter: &TokenBucketLimiter{Capacity: 10, RefillRate: 10.0 / 60.0, TTL: 30 * time.Minute}, // 10/min burst 10
		KeyFn: func(r *http.Request) string {
			p := strings.TrimSpace(s.Namespace)
			if p == "" {
				p = "dev"
			}
			return p + ":" + KeyByFingerprintOrIP(r)
		},
	})
	adminRL := (&RateLimitMiddleware{
		Limiter: &TokenBucketLimiter{Capacity: 60, RefillRate: 60.0 / 60.0, TTL: 30 * time.Minute}, // 60/min burst 60
		KeyFn: func(r *http.Request) string {
			p := strings.TrimSpace(s.Namespace)
			if p == "" {
				p = "dev"
			}
			return p + ":" + KeyByAdminTokenOrIP(r)
		},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/openapi.yaml", s.handleOpenAPISpec)
	mux.HandleFunc("/docs", s.handleOpenAPIDocs)
	// Feature flag: catalog_cache
	if _, ok := s.FeatureFlags["catalog_cache"]; ok {
		mux.Handle("/catalog", catalogCache.WrapGET(http.HandlerFunc(s.handleCatalog), func(r *http.Request) string {
			p := strings.TrimSpace(s.Namespace)
			if p == "" {
				p = "dev"
			}
			return p + ":catalog:" + r.URL.Path + "?" + canonicalQuery(r, "currency", "limit", "offset", "q", "style", "brewery", "abv_min", "abv_max")
		}))
		mux.Handle("/catalog/", catalogCache.WrapGET(http.HandlerFunc(s.handleCatalogDetail), func(r *http.Request) string {
			// sku is part of the path already.
			p := strings.TrimSpace(s.Namespace)
			if p == "" {
				p = "dev"
			}
			return p + ":catalog:" + r.URL.Path + "?" + canonicalQuery(r, "currency", "limit", "offset", "q", "style", "brewery", "abv_min", "abv_max")
		}))
	} else {
		mux.HandleFunc("/catalog", s.handleCatalog)
		mux.HandleFunc("/catalog/", s.handleCatalogDetail)
	}
	mux.Handle("/orders", s.requireSigned(http.HandlerFunc(s.handleOrders)))
	mux.Handle("/orders/", s.requireSigned(http.HandlerFunc(s.handleGetOrder)))
	mux.Handle("/customers/me", s.requireSigned(http.HandlerFunc(s.handleCustomerMe)))
	mux.Handle("/cart", s.requireSigned(http.HandlerFunc(s.handleCart)))
	mux.Handle("/shipping/estimate", s.requireSigned(http.HandlerFunc(s.handleShippingEstimate)))
	mux.Handle("/checkout", s.requireSigned(checkoutRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleCheckout)))))
	mux.Handle("/admin/products", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminProducts))))
	mux.Handle("/admin/products/", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminProductDetail))))
	mux.Handle("/admin/prices", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminPrices))))
	mux.Handle("/admin/inventory", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminInventory))))
	mux.HandleFunc("/admin/orders", s.handleAdminOrders)
	mux.HandleFunc("/admin/audit-events", s.handleAdminAuditEvents)

	if s.StripeWebhook != nil {
		path := s.StripeWebhookPath
		if strings.TrimSpace(path) == "" {
			path = "/webhooks/stripe"
		}
		mux.Handle(path, s.wrapWebhook(s.StripeWebhook))
	}

	h := http.Handler(mux)
	h = WithRequestID(h)
	h = AccessLog(metrics)(h)
	return h
}

func (s *Server) requireSigned(next http.Handler) http.Handler {
	// For local prototyping, if no secret is configured we accept unsigned requests.
	if strings.TrimSpace(s.CoreAPIHMACSecret) == "" && len(s.CoreAPIHMACKeys) == 0 {
		return next
	}
	if s.DB == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		})
	}
	keys := map[string][]byte{}
	for k, v := range s.CoreAPIHMACKeys {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		keys[strings.TrimSpace(k)] = []byte(strings.TrimSpace(v))
	}
	m := &HMACMiddleware{
		LegacySecret: []byte(s.CoreAPIHMACSecret),
		Secrets:      keys,
		Tolerance:    5 * time.Minute,
		Nonces:       &SQLNonceStore{DB: s.DB},
	}
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
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.PingContext(ctx); err != nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "not_ready", "db not ready")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.AdminToken == "" && len(s.AdminTokens) == 0 {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "admin not configured")
		return false
	}
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if provided == "" {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return false
	}

	// Allow rotation: ADMIN_TOKENS takes precedence, ADMIN_TOKEN is legacy.
	if len(s.AdminTokens) > 0 {
		ok := false
		for _, tok := range s.AdminTokens {
			if tok == "" {
				continue
			}
			if subtle.ConstantTimeCompare([]byte(provided), []byte(tok)) == 1 {
				ok = true
				break
			}
		}
		if !ok {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return false
		}
		return true
	}

	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.AdminToken)) != 1 {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return false
	}
	return true
}

func (s *Server) wrapWebhook(next http.Handler) http.Handler {
	// Basic hardening: method + body size limit.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleAdminProducts(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
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
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": list, "limit": limit, "offset": offset})
	case http.MethodPost:
		var req admin.UpsertProductInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		id, err := s.Admin.UpsertProduct(r.Context(), "admin", req)
		if err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusUnprocessableEntity
			}
			code := "internal"
			if status == http.StatusUnprocessableEntity {
				code = "invalid_input"
			}
			writeAPIError(w, r, status, code, "request invalid")
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
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/admin/products/")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req admin.UpsertProductInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		err := s.Admin.PutProductByID(r.Context(), "admin", domain.ProductID(id), req)
		if err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusUnprocessableEntity
			}
			if err == domain.ErrNotFound {
				status = http.StatusNotFound
			}
			code := "internal"
			switch status {
			case http.StatusNotFound:
				code = "not_found"
			case http.StatusUnprocessableEntity:
				code = "invalid_input"
			}
			writeAPIError(w, r, status, code, "request invalid")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPatch:
		var req admin.PatchProductInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		err := s.Admin.PatchProductByID(r.Context(), "admin", domain.ProductID(id), req)
		if err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusUnprocessableEntity
			}
			if err == domain.ErrNotFound {
				status = http.StatusNotFound
			}
			code := "internal"
			switch status {
			case http.StatusNotFound:
				code = "not_found"
			case http.StatusUnprocessableEntity:
				code = "invalid_input"
			}
			writeAPIError(w, r, status, code, "request invalid")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		reason := r.URL.Query().Get("reason")
		err := s.Admin.DeactivateProduct(r.Context(), "admin", domain.ProductID(id), reason)
		if err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusUnprocessableEntity
			}
			if err == domain.ErrNotFound {
				status = http.StatusNotFound
			}
			code := "internal"
			switch status {
			case http.StatusNotFound:
				code = "not_found"
			case http.StatusUnprocessableEntity:
				code = "invalid_input"
			}
			writeAPIError(w, r, status, code, "request invalid")
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
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut:
		var req admin.SetPriceInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		if req.Currency == "" {
			req.Currency = "COP"
		}
		if err := s.Admin.SetPrice(r.Context(), "admin", req); err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusUnprocessableEntity
			}
			code := "internal"
			if status == http.StatusUnprocessableEntity {
				code = "invalid_input"
			}
			writeAPIError(w, r, status, code, "request invalid")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		// End current price: ?product_id=...&currency=...&reason=...
		pid := strings.TrimSpace(r.URL.Query().Get("product_id"))
		if pid == "" {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing product_id")
			return
		}
		cur := strings.TrimSpace(r.URL.Query().Get("currency"))
		if cur == "" {
			cur = "COP"
		}
		reason := r.URL.Query().Get("reason")
		if err := s.Admin.EndCurrentPrice(r.Context(), "admin", domain.ProductID(pid), cur, reason); err != nil {
			status := http.StatusInternalServerError
			if err == domain.ErrInvalidInput {
				status = http.StatusUnprocessableEntity
			}
			if err == domain.ErrNotFound {
				status = http.StatusNotFound
			}
			code := "internal"
			switch status {
			case http.StatusNotFound:
				code = "not_found"
			case http.StatusUnprocessableEntity:
				code = "invalid_input"
			}
			writeAPIError(w, r, status, code, "request invalid")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleAdminInventory(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req admin.UpsertInventoryInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if err := s.Admin.UpsertInventory(r.Context(), "admin", req); err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrInvalidInput {
			status = http.StatusUnprocessableEntity
		}
		code := "internal"
		if status == http.StatusUnprocessableEntity {
			code = "invalid_input"
		}
		writeAPIError(w, r, status, code, "request invalid")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminAuditEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
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
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": evts, "limit": limit, "offset": offset})
}

func (s *Server) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status, err := orderStatusFilterOrEmpty(r.URL.Query().Get("status"))
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid status filter")
		return
	}
	createdFrom, err := parseOptionalRFC3339Query(r, "created_from")
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid created_from (RFC3339)")
		return
	}
	createdTo, err := parseOptionalRFC3339Query(r, "created_to")
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid created_to (RFC3339)")
		return
	}

	limit := parseLimitQuery(r, 100, 500)
	offset := parseOffsetQuery(r)

	orders, err := s.Admin.ListOrdersAdmin(r.Context(), admin.ListOrdersAdminInput{
		Status:      status,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"orders": orders, "limit": limit, "offset": offset})
}

func (s *Server) handleShippingEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	cur := strings.TrimSpace(r.URL.Query().Get("currency"))
	if cur == "" {
		cur = "COP"
	}
	canon, err := shipping.CanonicalCity(r.URL.Query().Get("city"))
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}
	key := shipping.CityKey(canon)
	n, err := shipping.LookupAmountMinor(r.Context(), s.DB, key, cur)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"city":                 canon,
		"currency":             cur,
		"shipping_minor_units": n,
	})
}

func parseOptionalRFC3339Query(r *http.Request, key string) (*time.Time, error) {
	s := strings.TrimSpace(r.URL.Query().Get(key))
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, domain.ErrInvalidInput
	}
	return &t, nil
}

func orderStatusFilterOrEmpty(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	switch domain.OrderStatus(v) {
	case domain.OrderStatusDraft, domain.OrderStatusPendingPayment, domain.OrderStatusPaid,
		domain.OrderStatusFulfilling, domain.OrderStatusShipped, domain.OrderStatusCancelled:
		return v, nil
	default:
		return "", domain.ErrInvalidInput
	}
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

func parseLimitQuery(r *http.Request, def, max int) int {
	n := parseIntDefault(r.URL.Query().Get("limit"), def)
	if n < 1 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func parseOffsetQuery(r *http.Request) int {
	n := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if n < 0 {
		return 0
	}
	return n
}

func parseOptionalFloatQuery(r *http.Request, key string) (*float64, error) {
	s := strings.TrimSpace(r.URL.Query().Get(key))
	if s == "" {
		return nil, nil
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return nil, domain.ErrInvalidInput
	}
	return &f, nil
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Catalog == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "COP"
	}
	abvMin, err := parseOptionalFloatQuery(r, "abv_min")
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid abv_min")
		return
	}
	abvMax, err := parseOptionalFloatQuery(r, "abv_max")
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid abv_max")
		return
	}

	limit := parseLimitQuery(r, 50, 100)
	offset := parseOffsetQuery(r)

	page, err := s.Catalog.ListActiveProducts(r.Context(), s.DB, catalog.ListCatalogFilter{
		Currency: currency,
		Limit:    limit,
		Offset:   offset,
		Query:    r.URL.Query().Get("q"),
		Style:    r.URL.Query().Get("style"),
		Brewery:  r.URL.Query().Get("brewery"),
		ABVMin:   abvMin,
		ABVMax:   abvMax,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"currency": currency,
		"items":    page.Items,
		"limit":    limit,
		"offset":   offset,
		"has_more": page.HasMore,
	})
}

func (s *Server) handleCatalogDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Catalog == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	sku := strings.TrimPrefix(r.URL.Path, "/catalog/")
	if sku == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing sku")
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
		code := "internal"
		if status == http.StatusNotFound {
			code = "not_found"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}
	price, err := s.Catalog.GetCurrentPrice(r.Context(), s.DB, p.ID, currency)
	if err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
		}
		code := "internal"
		if status == http.StatusNotFound {
			code = "not_found"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"product":       p,
		"price":         price,
		"available_qty": s.availableQty(r.Context(), p.ID),
	})
}

func (s *Server) availableQty(ctx context.Context, productID domain.ProductID) int {
	if s.DB == nil {
		return 0
	}
	row := s.DB.QueryRowContext(ctx, `
select greatest(0, coalesce(on_hand,0) - coalesce(reserved,0) - coalesce(safety_stock,0))
from inventory where product_id=$1
`, string(productID))
	var n int
	if err := row.Scan(&n); err != nil {
		return 0
	}
	return n
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Orders == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.lookupCustomerIDByFingerprint(r.Context(), fp)
	if err != nil {
		if err == domain.ErrNotFound {
			w.Header().Set("content-type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"orders": []any{}})
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	limit := parseLimitQuery(r, 20, 100)
	list, err := s.Orders.ListOrdersByCustomer(r.Context(), customerID, limit)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"orders": list})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing id")
		return
	}
	if s.Orders == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.lookupCustomerIDByFingerprint(r.Context(), fp)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	o, items, err := s.Orders.GetOrder(r.Context(), domain.OrderID(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
		}
		code := "internal"
		if status == http.StatusNotFound {
			code = "not_found"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}
	if o.CustomerID != customerID {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
		return
	}

	payload := map[string]any{
		"order": o,
		"items": items,
	}
	if sh, err := s.loadShipmentForOrder(r.Context(), o.ID); err == nil && sh != nil {
		payload["shipment"] = sh
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) loadShipmentForOrder(ctx context.Context, orderID domain.OrderID) (map[string]any, error) {
	if s.DB == nil || orderID == "" {
		return nil, nil
	}
	var carrier, tracking, status sql.NullString
	err := s.DB.QueryRowContext(ctx, `
select carrier, tracking_code, status
from shipments
where order_id = $1
`, string(orderID)).Scan(&carrier, &tracking, &status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{"Status": status.String}
	if carrier.Valid && carrier.String != "" {
		out["Carrier"] = carrier.String
	}
	if tracking.Valid && tracking.String != "" {
		out["TrackingCode"] = tracking.String
	}
	return out, nil
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

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
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
		if fp != "" && s.DB != nil {
			id, err := s.getOrCreateCustomerIDByFingerprint(r.Context(), fp)
			if err == nil {
				customerID = id
			}
		}
	}
	if customerID == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing customer_id (or X-SSH-Fingerprint)")
		return
	}
	if !req.PrivacyConsent {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "privacy_consent required")
		return
	}
	if strings.TrimSpace(req.Currency) == "" {
		req.Currency = "COP"
	}
	canonCity, err := shipping.CanonicalCity(req.City)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}
	req.City = canonCity

	cityKey := shipping.CityKey(canonCity)
	shipMinor, err := shipping.LookupAmountMinor(r.Context(), s.DB, cityKey, req.Currency)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	if err := s.saveCustomerProfile(r.Context(), customerID, req); err != nil {
		if err == domain.ErrInvalidInput {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid birth_date or age")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	_ = s.recordCustomerConsent(r.Context(), customerID, "privacy_policy")
	_ = s.saveCustomerCart(r.Context(), customerID, []cartLineJSON{})

	in := orders.CreatePendingOrderInput{
		CustomerID: customerID,
		Currency:   req.Currency,

		City:         req.City,
		Neighborhood: req.Neighborhood,
		AddressLine1: req.AddressLine1,
		Phone:        req.Phone,
		LegalID:      req.LegalID,
		FullName:     req.FullName,

		ShippingMinorUnits: shipMinor,
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
			SuccessURL:         req.SuccessURL,
			CancelURL:          req.CancelURL,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusBadGateway, "bad_gateway", "stripe error")
			return
		}

		_ = s.Orders.SetStripeRefs(r.Context(), o.ID, stripeOut.CheckoutSessionID, stripeOut.PaymentIntentID)
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order":  o,
		"items":  items,
		"stripe": stripeOut,
	})
}

func (s *Server) loadCustomer(ctx context.Context, id domain.CustomerID) (*domain.Customer, error) {
	row := s.DB.QueryRowContext(ctx, `
select id, ssh_fingerprint, email, phone, legal_id, full_name, birth_date,
       address_line1, neighborhood, city, created_at, updated_at
from customers where id=$1
`, string(id))
	var c domain.Customer
	var email, phone, legalID, fullName, addr, hood, city sql.NullString
	var birth sql.NullTime
	if err := row.Scan(
		&c.ID, &c.SSHFingerprint, &email, &phone, &legalID, &fullName, &birth,
		&addr, &hood, &city, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	c.Email = email.String
	c.Phone = phone.String
	c.LegalID = legalID.String
	c.FullName = fullName.String
	c.AddressLine1 = addr.String
	c.Neighborhood = hood.String
	c.City = city.String
	if birth.Valid {
		c.BirthDate = birth.Time
	}
	return &c, nil
}

func (s *Server) saveCustomerProfile(ctx context.Context, id domain.CustomerID, req checkoutRequest) error {
	if s.DB == nil {
		return nil
	}
	var birth any
	if strings.TrimSpace(req.BirthDate) != "" {
		bd, err := parseBirthDate(req.BirthDate)
		if err != nil {
			return domain.ErrInvalidInput
		}
		if !age.IsAdult(bd, time.Now()) {
			return domain.ErrInvalidInput
		}
		birth = bd
	}
	_, err := s.DB.ExecContext(ctx, `
update customers
set city=$2, neighborhood=$3, address_line1=$4, phone=$5, legal_id=$6, full_name=$7,
    birth_date=coalesce($8, birth_date), updated_at=now()
where id=$1
`, string(id), req.City, req.Neighborhood, req.AddressLine1, req.Phone, req.LegalID, req.FullName, birth)
	return err
}

func (s *Server) lookupCustomerIDByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error) {
	if fp == "" {
		return "", domain.ErrInvalidInput
	}
	row := s.DB.QueryRowContext(ctx, `select id from customers where ssh_fingerprint=$1`, fp)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return domain.CustomerID(id), nil
}

func (s *Server) getOrCreateCustomerIDByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error) {
	if fp == "" {
		return "", domain.ErrInvalidInput
	}
	if id, err := s.lookupCustomerIDByFingerprint(ctx, fp); err == nil {
		return id, nil
	} else if err != domain.ErrNotFound {
		return "", err
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
	var id string
	row2 := s.DB.QueryRowContext(ctx, `select id from customers where ssh_fingerprint=$1`, fp)
	if err := row2.Scan(&id); err != nil {
		return "", err
	}
	return domain.CustomerID(id), nil
}
