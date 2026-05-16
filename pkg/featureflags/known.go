package featureflags

// Known feature flag names for core-api (FEATURE_FLAGS env, comma-separated).
// Names are case-insensitive when parsed.
const (
	CatalogCache      = "catalog_cache"
	CatalogCacheRedis = "catalog_cache_redis"
	RedisRateLimit    = "redis_rate_limit"
	EmailPaymentLink  = "email_payment_link"
	EmailReceipt      = "email_receipt"
)

// Known describes a single flag for operators and docs.
type Known struct {
	Name        string
	Description string
	Requires    string // optional dependency note
}

// All returns the canonical flag catalog (source of truth for README).
func All() []Known {
	return []Known{
		{
			Name:        CatalogCache,
			Description: "SWR cache for GET /catalog and GET /catalog/{sku} (in-process; per replica).",
		},
		{
			Name:        CatalogCacheRedis,
			Description: "Same as catalog_cache but backed by Redis (shared across core-api replicas).",
			Requires:    "REDIS_URL",
		},
		{
			Name:        RedisRateLimit,
			Description: "Shared token-bucket rate limits for POST /checkout and /admin/* (via Redis).",
			Requires:    "REDIS_URL",
		},
		{
			Name:        EmailPaymentLink,
			Description: "Send payment-link email after checkout when customer email is known.",
			Requires:    "EMAIL_PROVIDER (log or smtp)",
		},
		{
			Name:        EmailReceipt,
			Description: "Send receipt email when order.paid is consumed (nats-consume worker).",
			Requires:    "EMAIL_PROVIDER; OUTBOX_PUBLISHER=nats and nats-consume running",
		},
	}
}
