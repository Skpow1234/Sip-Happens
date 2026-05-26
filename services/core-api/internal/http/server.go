package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/config"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/natshealth"
	"sip-happens/pkg/obslog"
	"sip-happens/pkg/redact"
	"sip-happens/pkg/shipping"
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/email"
	"sip-happens/services/core-api/internal/invoicing"
	"sip-happens/services/core-api/internal/oauthapps"
	"sip-happens/services/core-api/internal/objectstore"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/subscriptions"
	"sip-happens/services/core-api/internal/tokens"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	Orders        orders.Service
	Subscriptions subscriptions.Repository
	Tokens        tokens.Repository
	OAuthClients  oauthapps.ClientRepository
	OAuthCodes    oauthapps.CodeRepository
	Payments      payments.Repository
	Customers     customers.Service
	Addresses     addresses.Repository
	Catalog       catalog.Repository
	Admin         admin.Service
	Invoices      *invoicing.Service
	AdminToken    string
	AdminTokens   []string
	// AdminTokenRoles maps bearer token string to RBAC tier when using ADMIN_* role envs (optional).
	AdminTokenRoles   map[string]AdminRole
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

	Email *email.Service

	// Redis is optional; used when feature flags request shared cache or rate limits.
	Redis *redis.Client
	// RedisRequired makes /readyz fail when Redis is configured but unreachable.
	RedisRequired bool

	// OutboxPublisher / NATSURL gate /readyz NATS connectivity (OUTBOX_PUBLISHER=nats).
	OutboxPublisher config.OutboxPublisherKind
	NATSURL         string

	ProductImages objectstore.ProductImages

	PrivacyPolicyURL        string
	ConsumerRightsURL       string
	DataProcessingPolicyURL string
}

func (s *Server) Handler() http.Handler {
	metrics := NewMetrics()

	var idemStore IdempotencyStore
	if s.DB != nil {
		idemStore = &SQLIdempotencyStore{DB: s.DB}
	}
	idem := &IdempotencyMiddleware{Store: idemStore, TTL: 24 * time.Hour, Prefix: strings.TrimSpace(s.Namespace)}

	nsPrefix := strings.TrimSpace(s.Namespace)
	if nsPrefix == "" {
		nsPrefix = "dev"
	}

	var catalogCache *SWRCache
	if s.featureEnabled("catalog_cache_redis") && s.Redis != nil {
		catalogCache = NewRedisSWRCache(s.Redis, nsPrefix, 15*time.Second, 45*time.Second)
	} else {
		catalogCache = NewSWRCache(15*time.Second, 45*time.Second)
	}

	// Rate limits use `N / 60.0` to read as "N tokens per minute" so the
	// pair (checkout = 10/min, admin = 60/min) stays visually parallel.
	// staticcheck flags 60.0/60.0 as identical-sides (SA4000) which is true
	// but defeats the readability — we keep the form intentionally.
	checkoutLimiter := RateLimiter(&TokenBucketLimiter{Capacity: 10, RefillRate: 10.0 / 60.0, TTL: 30 * time.Minute})
	//lint:ignore SA4000 60.0/60.0 = "60 per minute"; keeps parity with the 10.0/60.0 sibling above
	adminLimiter := RateLimiter(&TokenBucketLimiter{Capacity: 60, RefillRate: 60.0 / 60.0, TTL: 30 * time.Minute})
	if s.featureEnabled("redis_rate_limit") && s.Redis != nil {
		rlPrefix := fmt.Sprintf("sip:%s:rl", nsPrefix)
		checkoutLimiter = &RedisTokenBucketLimiter{
			Client: s.Redis, KeyPrefix: rlPrefix,
			Capacity: 10, RefillRate: 10.0 / 60.0, TTL: 30 * time.Minute,
		}
		adminLimiter = &RedisTokenBucketLimiter{
			Client: s.Redis, KeyPrefix: rlPrefix,
			//lint:ignore SA4000 60.0/60.0 = "60 per minute"; keeps parity with sibling limiters
			Capacity: 60, RefillRate: 60.0 / 60.0, TTL: 30 * time.Minute,
		}
	}

	checkoutRL := (&RateLimitMiddleware{
		Limiter: checkoutLimiter,
		KeyFn: func(r *http.Request) string {
			p := strings.TrimSpace(s.Namespace)
			if p == "" {
				p = "dev"
			}
			return p + ":" + KeyByFingerprintOrIP(r)
		},
	})
	adminRL := (&RateLimitMiddleware{
		Limiter: adminLimiter,
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
	mux.HandleFunc("/admin/console", s.handleAdminConsole)
	// Feature flag: catalog_cache (in-memory) or catalog_cache_redis (shared SWR; requires REDIS_URL).
	if s.featureEnabled("catalog_cache") || s.featureEnabled("catalog_cache_redis") {
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
	mux.Handle("GET /orders", s.requireSigned(http.HandlerFunc(s.handleOrders)))
	mux.Handle("POST /orders", s.requireSigned(checkoutRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleOrderCreate)))))
	mux.Handle("/orders/", s.requireSigned(http.HandlerFunc(s.handleGetOrder)))
	mux.Handle("GET /subscriptions", s.requireSigned(http.HandlerFunc(s.handleSubscriptions)))
	mux.Handle("POST /subscriptions", s.requireSigned(checkoutRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleSubscriptionCreate)))))
	mux.Handle("/subscriptions/", s.requireSigned(http.HandlerFunc(s.handleSubscriptionByID)))
	mux.Handle("/tokens", s.requireSigned(http.HandlerFunc(s.handleTokens)))
	mux.Handle("/tokens/", s.requireSigned(http.HandlerFunc(s.handleTokenByID)))
	mux.Handle("POST /oauth/authorize", s.requireSigned(checkoutRL.Wrap(http.HandlerFunc(s.handleOAuthAuthorize))))
	mux.HandleFunc("POST /oauth/token", s.handleOAuthToken)
	mux.Handle("/oauth/clients", s.requireSigned(http.HandlerFunc(s.handleOAuthClients)))
	mux.Handle("/oauth/clients/", s.requireSigned(http.HandlerFunc(s.handleOAuthClientByID)))
	mux.Handle("GET /view/init", s.requireSigned(http.HandlerFunc(s.handleViewInit)))
	mux.Handle("/customers/me", s.requireSigned(http.HandlerFunc(s.handleCustomerMe)))
	mux.Handle("/profile", s.requireSigned(http.HandlerFunc(s.handleProfile)))
	mux.Handle("/addresses", s.requireSigned(http.HandlerFunc(s.handleAddresses)))
	mux.Handle("/addresses/", s.requireSigned(http.HandlerFunc(s.handleAddressByID)))
	mux.Handle("POST /payment-methods/collect", s.requireSigned(http.HandlerFunc(s.handlePaymentMethodsCollect)))
	mux.Handle("/payment-methods", s.requireSigned(http.HandlerFunc(s.handlePaymentMethods)))
	mux.Handle("/payment-methods/", s.requireSigned(http.HandlerFunc(s.handlePaymentMethodByID)))
	mux.Handle("/cart/items/", s.requireSigned(http.HandlerFunc(s.handleCartItemByID)))
	mux.Handle("/cart/address", s.requireSigned(http.HandlerFunc(s.handleCartAddress)))
	mux.Handle("/cart/payment-method", s.requireSigned(http.HandlerFunc(s.handleCartPaymentMethod)))
	mux.Handle("/cart/convert", s.requireSigned(checkoutRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleCartConvert)))))
	mux.Handle("/cart", s.requireSigned(http.HandlerFunc(s.handleCart)))
	mux.Handle("/shipping/estimate", s.requireSigned(http.HandlerFunc(s.handleShippingEstimate)))
	mux.Handle("/checkout", s.requireSigned(checkoutRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleCheckout)))))
	mux.Handle("/admin/products", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminProducts))))
	mux.Handle("/admin/products/", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminProductDetail))))
	mux.Handle("POST /admin/products/{id}/image", adminRL.Wrap(http.HandlerFunc(s.handleAdminProductImage)))
	mux.Handle("/admin/prices", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminPrices))))
	mux.Handle("/admin/inventory/reservations", adminRL.Wrap(http.HandlerFunc(s.handleAdminInventoryReservations)))
	mux.Handle("/admin/inventory", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminInventory))))
	mux.Handle("POST /admin/orders/{id}/cancel", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminOrderCancel))))
	mux.Handle("POST /admin/orders/{id}/reconcile-stripe", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminOrderReconcileStripe))))
	mux.Handle("POST /admin/orders/{id}/invoice", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminOrderIssueInvoice))))
	mux.Handle("PATCH /admin/orders/{id}/shipment", adminRL.Wrap(idem.Wrap(http.HandlerFunc(s.handleAdminOrderShipment))))
	mux.HandleFunc("GET /admin/orders/export", s.handleAdminOrdersExport)
	mux.HandleFunc("/admin/orders", s.handleAdminOrders)
	mux.HandleFunc("/admin/orders/", s.handleAdminOrderDetail)
	mux.HandleFunc("/admin/audit-events", s.handleAdminAuditEvents)
	mux.HandleFunc("/admin/stripe/dead-letters", s.handleAdminStripeDeadLetters)
	mux.HandleFunc("POST /admin/stripe/dead-letters/replay", s.handleAdminStripeDeadLetterReplay)

	if s.StripeWebhook != nil {
		path := s.StripeWebhookPath
		if strings.TrimSpace(path) == "" {
			path = "/webhooks/stripe"
		}
		mux.Handle(path, s.wrapWebhook(s.StripeWebhook))
	}

	h := http.Handler(mux)
	h = WithRequestID(h)
	h = WithCustomerContext(h)
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
	if s.Redis != nil {
		if err := s.Redis.Ping(ctx).Err(); err != nil {
			writeAPIError(w, r, http.StatusServiceUnavailable, "not_ready", "redis not ready")
			return
		}
	} else if s.RedisRequired {
		writeAPIError(w, r, http.StatusServiceUnavailable, "not_ready", "redis not configured")
		return
	}
	if s.OutboxPublisher == config.OutboxPublisherNATS {
		if strings.TrimSpace(s.NATSURL) == "" {
			writeAPIError(w, r, http.StatusServiceUnavailable, "not_ready", "nats not configured")
			return
		}
		if err := natshealth.Ping(ctx, s.NATSURL); err != nil {
			writeAPIError(w, r, http.StatusServiceUnavailable, "not_ready", "nats not ready")
			return
		}
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
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
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !s.requireAdminRole(w, r, AdminRoleFinance) {
			return
		}
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
		if !s.requireAdminRole(w, r, AdminRoleOps) {
			return
		}
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
	if !s.requireAdminRole(w, r, AdminRoleOps) {
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
	productID := domain.ProductID(id)
	switch r.Method {
	case http.MethodPut:
		if !s.checkAdminProductIfMatch(w, r, productID) {
			return
		}
		var req admin.UpsertProductInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		err := s.Admin.PutProductByID(r.Context(), "admin", productID, req)
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
		s.writeAdminProductNoContent(w, r, productID)
	case http.MethodPatch:
		if !s.checkAdminProductIfMatch(w, r, productID) {
			return
		}
		var req admin.PatchProductInput
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
			return
		}
		err := s.Admin.PatchProductByID(r.Context(), "admin", productID, req)
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
		s.writeAdminProductNoContent(w, r, productID)
	case http.MethodDelete:
		reason := r.URL.Query().Get("reason")
		err := s.Admin.DeactivateProduct(r.Context(), "admin", productID, reason)
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
	if !s.requireAdminRole(w, r, AdminRoleOps) {
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
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !s.requireAdminRole(w, r, AdminRoleFinance) {
			return
		}
		limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
		offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
		var productID domain.ProductID
		if v := strings.TrimSpace(r.URL.Query().Get("product_id")); v != "" {
			productID = domain.ProductID(v)
		}
		list, err := s.Admin.ListInventory(r.Context(), admin.ListInventoryInput{
			ProductID: productID,
			SKU:       strings.TrimSpace(r.URL.Query().Get("sku")),
			Limit:     limit,
			Offset:    offset,
		})
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": list, "limit": limit, "offset": offset})
		return
	case http.MethodPost, http.MethodPut:
		if !s.requireAdminRole(w, r, AdminRoleOps) {
			return
		}
	default:
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

func (s *Server) handleAdminInventoryReservations(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
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
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	var productID domain.ProductID
	if v := strings.TrimSpace(r.URL.Query().Get("product_id")); v != "" {
		productID = domain.ProductID(v)
	}
	list, err := s.Admin.ListActiveReservations(r.Context(), admin.ListActiveReservationsInput{
		ProductID: productID,
		SKU:       strings.TrimSpace(r.URL.Query().Get("sku")),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"reservations": list, "limit": limit, "offset": offset})
}

func (s *Server) handleAdminAuditEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
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
	orderID := strings.TrimSpace(r.URL.Query().Get("order_id"))

	evts, err := s.Admin.ListAuditEvents(r.Context(), admin.ListAuditEventsInput{
		AggregateType: aggType,
		AggregateID:   aggID,
		OrderID:       orderID,
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
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
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

func (s *Server) handleAdminOrderDetail(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleFinance) {
		return
	}
	if s.Admin == nil || s.Orders == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/admin/orders/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeAPIError(w, r, http.StatusNotFound, "not_found", "not found")
		return
	}
	orderID := domain.OrderID(id)

	o, items, err := s.Orders.GetOrder(r.Context(), orderID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
			code = "not_found"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}

	paymentRows, err := s.Admin.ListPaymentsForOrder(r.Context(), orderID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	statusHistory, err := s.Admin.ListPaymentStatusHistoryForOrder(r.Context(), orderID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	payload := map[string]any{
		"order":                  o,
		"items":                  items,
		"payments":               paymentRows,
		"payment_status_history": statusHistory,
	}
	if sh, err := s.loadShipmentForOrder(r.Context(), o.ID); err == nil && sh != nil {
		payload["shipment"] = sh
	}
	if s.Invoices != nil {
		if inv, err := s.Invoices.GetByOrderID(r.Context(), orderID); err == nil && inv != nil {
			payload["invoice"] = inv
		} else if err != nil && err != domain.ErrNotFound {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
	}

	s.recordAdminPIIAccess(r, "admin_order_detail", "order", string(orderID), map[string]string{
		"customer_id": string(o.CustomerID),
	})

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleAdminOrderCancel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleSupport) {
		return
	}
	if s.Admin == nil || s.Orders == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing id")
		return
	}
	orderID := domain.OrderID(id)

	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "admin_cancel"
	}

	if err := s.Orders.CancelPendingOrder(r.Context(), orderID); err != nil {
		switch err {
		case domain.ErrNotFound:
			writeAPIError(w, r, http.StatusNotFound, "not_found", "not found")
		case domain.ErrConflict:
			writeAPIError(w, r, http.StatusConflict, "conflict", "order cannot be cancelled in current status")
		default:
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	actor := adminActorFromRequest(r)
	payload, _ := json.Marshal(map[string]string{"reason": reason})
	if err := s.Admin.RecordAudit(r.Context(), actor, "order", string(orderID), "order.cancelled", string(payload)); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	o, items, err := s.Orders.GetOrder(r.Context(), orderID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"order": o, "items": items})
}

func (s *Server) handleAdminOrderReconcileStripe(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminRole(w, r, AdminRoleOps) {
		return
	}
	if s.Orders == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	if s.Stripe == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "misconfigured", "stripe not configured")
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing id")
		return
	}
	orderID := domain.OrderID(id)

	res, err := stripepayments.ManualReconcilePendingOrder(r.Context(), s.Orders, s.Stripe, orderID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "not found")
			return
		}
		if errors.Is(err, domain.ErrInvalidInput) {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "missing stripe_payment_intent_id")
			return
		}
		writeAPIError(w, r, http.StatusBadGateway, "bad_gateway", redact.String(err.Error()))
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
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

	limit, err := parseCatalogLimitQuery(r)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}
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
		Featured: parseBoolQuery(r, "featured"),
	})
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	items := make([]catalog.ItemView, 0, len(page.Items))
	for _, it := range page.Items {
		items = append(items, catalog.ItemViewFromProductWithPrice(it))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"currency": currency,
		"items":    items,
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
	ref := strings.TrimPrefix(r.URL.Path, "/catalog/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing product id")
		return
	}

	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "COP"
	}

	p, err := catalog.ResolveProduct(r.Context(), s.Catalog, s.DB, ref)
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

	item := catalog.ItemView{
		Product:      catalog.ProductViewFromDomain(*p),
		Price:        catalog.PriceViewFromDomain(*price),
		AvailableQty: s.availableQty(r.Context(), p.ID),
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"currency":      currency,
		"product":       item.Product,
		"price":         item.Price,
		"available_qty": item.AvailableQty,
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

func (s *Server) featureEnabled(name string) bool {
	if s.FeatureFlags == nil {
		return false
	}
	_, ok := s.FeatureFlags[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func (s *Server) maybeSendPaymentLinkEmail(o *domain.Order, checkoutFullName, paymentURL string) {
	if s.Email == nil || !s.featureEnabled("email_payment_link") || o == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		name := strings.TrimSpace(checkoutFullName)
		emailAddr := ""
		if s.Customers != nil {
			cust, err := s.Customers.GetByID(ctx, o.CustomerID)
			if err == nil && cust != nil {
				if name == "" {
					name = cust.FullName
				}
				emailAddr = cust.Email
			}
		}
		logCtx := obslog.WithOrderID(ctx, string(o.ID))
		logCtx = obslog.WithCustomerID(logCtx, string(o.CustomerID))
		if err := s.Email.SendPaymentLink(ctx, o, emailAddr, name, paymentURL); err != nil {
			obslog.Warn(logCtx, "payment_link_email_failed", map[string]any{"err": err.Error()})
		}
	}()
}
