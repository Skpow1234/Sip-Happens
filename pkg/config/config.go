package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"sip-happens/pkg/featureflags"
)

type CoreAPI struct {
	HTTPAddr  string
	Namespace string
	Flags     featureflags.Flags

	PostgresDSN    string
	MigrationsPath string
	DBPool         DBPool

	StripeSecretKey     string
	StripeWebhookSecret string
	StripeWebhookPath   string
	StripeMock          bool

	AdminToken  string
	AdminTokens []string
	// AdminSupportTokens / AdminFinanceTokens assign RBAC tiers (roadmap-040).
	// Tokens listed only in AdminTokens (or legacy AdminToken) default to ops.
	AdminSupportTokens []string
	AdminFinanceTokens []string

	// CoreAPIHMACSecret is legacy v1 signing shared secret.
	CoreAPIHMACSecret string
	// CoreAPIHMACKeyID and CoreAPIHMACKeys enable v2 signing (key-id + rotation).
	CoreAPIHMACKeyID string
	CoreAPIHMACKeys  map[string]string

	RedisURL      string
	RedisRequired bool

	MinIO MinIO

	// Legal URLs for emails and admin console (DESIGN §5.3–§5.4).
	PrivacyPolicyURL        string
	ConsumerRightsURL       string
	DataProcessingPolicyURL string
}

func LoadCoreAPI() (CoreAPI, error) {
	var c CoreAPI

	c.HTTPAddr = getenvDefault("HTTP_ADDR", ":8080")
	c.Namespace = getenvDefault("NAMESPACE", "dev")
	c.Flags = featureflags.Parse(os.Getenv("FEATURE_FLAGS"))
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}

	c.MigrationsPath = getenvDefault("MIGRATIONS_PATH", "infra/migrations")
	c.DBPool = loadDBPool()

	c.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	c.StripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
	c.StripeWebhookPath = getenvDefault("STRIPE_WEBHOOK_PATH", "/webhooks/stripe")
	c.StripeMock = getenvBoolDefault("STRIPE_MOCK", true)

	c.AdminToken = os.Getenv("ADMIN_TOKEN")
	if v := strings.TrimSpace(os.Getenv("ADMIN_TOKENS")); v != "" {
		for _, tok := range strings.Split(v, ",") {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				c.AdminTokens = append(c.AdminTokens, tok)
			}
		}
	}
	c.AdminSupportTokens = splitCommaEnv("ADMIN_SUPPORT_TOKENS")
	c.AdminFinanceTokens = splitCommaEnv("ADMIN_FINANCE_TOKENS")
	c.CoreAPIHMACSecret = os.Getenv("CORE_API_HMAC_SECRET")
	c.CoreAPIHMACKeyID = strings.TrimSpace(os.Getenv("CORE_API_HMAC_KEY_ID"))
	c.CoreAPIHMACKeys = parseKeyRing(os.Getenv("CORE_API_HMAC_KEYS"))
	c.RedisURL = strings.TrimSpace(os.Getenv("REDIS_URL"))
	c.RedisRequired = RedisRequiredForReadyz()
	c.MinIO = LoadMinIO()
	c.PrivacyPolicyURL = getenvDefault("PRIVACY_POLICY_URL", "https://sip-happens.local/privacy")
	c.ConsumerRightsURL = getenvDefault("CONSUMER_RIGHTS_URL", "https://www.sic.gov.co/proteccion-del-consumidor")
	c.DataProcessingPolicyURL = getenvDefault("DATA_PROCESSING_POLICY_URL", "https://sip-happens.local/legal/data-processing")

	return c, nil
}

// AdminTokenRoleMap returns bearer token -> coarse role label ("ops", "support", "finance").
// Highest role wins if a token appears in multiple lists. Used by core-api HTTP layer.
func (c CoreAPI) AdminTokenRoleMap() map[string]string {
	out := make(map[string]string)
	set := func(tok, role string) {
		if tok == "" {
			return
		}
		cur := out[tok]
		if roleRank(role) > roleRank(cur) {
			out[tok] = role
		}
	}
	for _, t := range c.AdminTokens {
		set(t, "ops")
	}
	set(strings.TrimSpace(c.AdminToken), "ops")
	for _, t := range c.AdminSupportTokens {
		set(t, "support")
	}
	for _, t := range c.AdminFinanceTokens {
		set(t, "finance")
	}
	return out
}

func roleRank(role string) int {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "ops", "admin":
		return 3
	case "support":
		return 2
	case "finance":
		return 1
	default:
		return 0
	}
}

func splitCommaEnv(key string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type DBPool struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
}

func loadDBPool() DBPool {
	return DBPool{
		MaxOpenConns:    getenvIntDefault("DB_MAX_OPEN_CONNS", 10),
		MaxIdleConns:    getenvIntDefault("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: time.Duration(getenvIntDefault("DB_CONN_MAX_LIFETIME_SECONDS", 1800)) * time.Second, // 30m
		ConnMaxIdleTime: time.Duration(getenvIntDefault("DB_CONN_MAX_IDLE_SECONDS", 300)) * time.Second,      // 5m
		PingTimeout:     time.Duration(getenvIntDefault("DB_PING_TIMEOUT_SECONDS", 2)) * time.Second,
	}
}

type CancelExpiredJob struct {
	PostgresDSN string
	Interval    time.Duration
	BatchLimit  int
	RunOnce     bool
	DBPool      DBPool
}

func LoadCancelExpiredJob() (CancelExpiredJob, error) {
	var c CancelExpiredJob
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}
	c.Interval = time.Duration(getenvIntDefault("JOB_INTERVAL_SECONDS", 30)) * time.Second
	c.BatchLimit = getenvIntDefault("JOB_BATCH_LIMIT", 100)
	c.RunOnce = getenvBoolDefault("JOB_RUN_ONCE", false)
	c.DBPool = loadDBPool()
	if c.BatchLimit <= 0 || c.BatchLimit > 1000 {
		return c, fmt.Errorf("JOB_BATCH_LIMIT must be between 1 and 1000")
	}
	if c.Interval <= 0 {
		return c, fmt.Errorf("JOB_INTERVAL_SECONDS must be > 0")
	}
	return c, nil
}

type StripeReconcileJob struct {
	PostgresDSN string
	Interval    time.Duration
	BatchLimit  int
	RunOnce     bool
	DBPool      DBPool

	StripeSecretKey string
	StripeMock      bool
}

func LoadStripeReconcileJob() (StripeReconcileJob, error) {
	var c StripeReconcileJob
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}
	c.Interval = time.Duration(getenvIntDefault("JOB_INTERVAL_SECONDS", 60)) * time.Second
	c.BatchLimit = getenvIntDefault("JOB_BATCH_LIMIT", 100)
	c.RunOnce = getenvBoolDefault("JOB_RUN_ONCE", false)
	c.DBPool = loadDBPool()
	if c.BatchLimit <= 0 || c.BatchLimit > 1000 {
		return c, fmt.Errorf("JOB_BATCH_LIMIT must be between 1 and 1000")
	}
	if c.Interval <= 0 {
		return c, fmt.Errorf("JOB_INTERVAL_SECONDS must be > 0")
	}

	c.StripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
	c.StripeMock = getenvBoolDefault("STRIPE_MOCK", true)

	if c.StripeSecretKey == "" && !c.StripeMock {
		return c, fmt.Errorf("STRIPE_SECRET_KEY is required when STRIPE_MOCK=false")
	}
	return c, nil
}

type OutboxDispatchJob struct {
	PostgresDSN string
	Interval    time.Duration
	BatchLimit  int
	RunOnce     bool
	DBPool      DBPool

	LockedBy    string
	MaxAttempts int
	BaseBackoff time.Duration
}

func LoadOutboxDispatchJob() (OutboxDispatchJob, error) {
	var c OutboxDispatchJob
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}
	c.Interval = time.Duration(getenvIntDefault("JOB_INTERVAL_SECONDS", 5)) * time.Second
	c.BatchLimit = getenvIntDefault("JOB_BATCH_LIMIT", 100)
	c.RunOnce = getenvBoolDefault("JOB_RUN_ONCE", false)
	c.DBPool = loadDBPool()
	c.LockedBy = getenvDefault("OUTBOX_LOCKED_BY", "outbox-dispatcher")
	c.MaxAttempts = getenvIntDefault("OUTBOX_MAX_ATTEMPTS", 10)
	c.BaseBackoff = time.Duration(getenvIntDefault("OUTBOX_BASE_BACKOFF_SECONDS", 30)) * time.Second

	if c.BatchLimit <= 0 || c.BatchLimit > 1000 {
		return c, fmt.Errorf("JOB_BATCH_LIMIT must be between 1 and 1000")
	}
	if c.Interval <= 0 {
		return c, fmt.Errorf("JOB_INTERVAL_SECONDS must be > 0")
	}
	if c.MaxAttempts <= 0 || c.MaxAttempts > 100 {
		return c, fmt.Errorf("OUTBOX_MAX_ATTEMPTS must be between 1 and 100")
	}
	if c.BaseBackoff <= 0 {
		return c, fmt.Errorf("OUTBOX_BASE_BACKOFF_SECONDS must be > 0")
	}
	return c, nil
}

type OutboxReplayJob struct {
	PostgresDSN string
	Limit       int
	RunOnce     bool
	DBPool      DBPool
}

func LoadOutboxReplayJob() (OutboxReplayJob, error) {
	var c OutboxReplayJob
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}
	c.Limit = getenvIntDefault("OUTBOX_REPLAY_LIMIT", 100)
	c.RunOnce = getenvBoolDefault("JOB_RUN_ONCE", true)
	c.DBPool = loadDBPool()
	if c.Limit <= 0 || c.Limit > 10000 {
		return c, fmt.Errorf("OUTBOX_REPLAY_LIMIT must be between 1 and 10000")
	}
	return c, nil
}

type InventoryReconcileJob struct {
	PostgresDSN string
	Interval    time.Duration
	BatchLimit  int
	RunOnce     bool
	Fix         bool
	ExpireGrace time.Duration
	DBPool      DBPool
}

func LoadInventoryReconcileJob() (InventoryReconcileJob, error) {
	var c InventoryReconcileJob
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}
	c.Interval = time.Duration(getenvIntDefault("JOB_INTERVAL_SECONDS", 300)) * time.Second
	c.BatchLimit = getenvIntDefault("JOB_BATCH_LIMIT", 500)
	c.RunOnce = getenvBoolDefault("JOB_RUN_ONCE", false)
	c.Fix = getenvBoolDefault("INV_RECONCILE_FIX", false)
	c.ExpireGrace = time.Duration(getenvIntDefault("INV_EXPIRE_GRACE_SECONDS", 300)) * time.Second
	c.DBPool = loadDBPool()
	if c.BatchLimit <= 0 || c.BatchLimit > 10000 {
		return c, fmt.Errorf("JOB_BATCH_LIMIT must be between 1 and 10000")
	}
	if c.Interval <= 0 {
		return c, fmt.Errorf("JOB_INTERVAL_SECONDS must be > 0")
	}
	if c.ExpireGrace < 0 {
		c.ExpireGrace = 0
	}
	return c, nil
}

type SSHGateway struct {
	SSHAddr     string
	CoreAPIURL  string
	HostKeyPath string

	// RequireAuthorizedKeys enforces SSH_AUTHORIZED_KEYS_PATH (set in production).
	RequireAuthorizedKeys bool
	AuthorizedKeysPath    string
	// DevFingerprint is used when the client has no public key (dev only; empty disables).
	DevFingerprint string
	// PrivacyPolicyURL is shown in the TUI welcome and checkout consent screens.
	PrivacyPolicyURL string
	// ConsumerRightsURL is the SIC consumer-protection link shown in the TUI (DESIGN §5.3).
	ConsumerRightsURL string
	// DataProcessingPolicyURL is linked from TUI checkout consent (Ley 1581).
	DataProcessingPolicyURL string
	// SupportContact is shown on the post-payment confirmation screen.
	SupportContact string
	// DispatchETAPolicy describes expected dispatch timing (confirmation screen).
	DispatchETAPolicy string
	// MetricsAddr is an optional listen address for Prometheus (e.g. :9101).
	MetricsAddr string
	// DisablePasswordAuth rejects SSH password logins (recommended in production).
	DisablePasswordAuth bool
	// DisablePortForwarding denies SSH local/reverse port forwarding.
	DisablePortForwarding bool

	// CoreAPIHMACSecret is legacy v1 signing shared secret.
	CoreAPIHMACSecret string
	// CoreAPIHMACKeyID and CoreAPIHMACKeys enable v2 signing (key-id + rotation).
	CoreAPIHMACKeyID string
	CoreAPIHMACKeys  map[string]string
}

func LoadSSHGateway() (SSHGateway, error) {
	var c SSHGateway
	c.SSHAddr = getenvDefault("SSH_ADDR", "0.0.0.0:2222")
	c.CoreAPIURL = getenvDefault("CORE_API_URL", "http://core-api:8080")
	c.HostKeyPath = getenvDefault("SSH_HOST_KEY_PATH", "/data/ssh_host_ed25519_key")
	c.RequireAuthorizedKeys = getenvBoolDefault("SSH_REQUIRE_AUTHORIZED_KEYS", false)
	c.AuthorizedKeysPath = getenvDefault("SSH_AUTHORIZED_KEYS_PATH", "/data/authorized_keys")
	c.DevFingerprint = strings.TrimSpace(os.Getenv("SSH_DEV_FINGERPRINT"))
	c.PrivacyPolicyURL = getenvDefault("PRIVACY_POLICY_URL", "https://sip-happens.local/privacy")
	c.ConsumerRightsURL = getenvDefault("CONSUMER_RIGHTS_URL", "https://www.sic.gov.co/proteccion-del-consumidor")
	c.DataProcessingPolicyURL = getenvDefault("DATA_PROCESSING_POLICY_URL", "https://sip-happens.local/legal/data-processing")
	c.SupportContact = getenvDefault("SUPPORT_CONTACT", "soporte@sip-happens.local")
	c.DispatchETAPolicy = getenvDefault("DISPATCH_ETA_POLICY", "Despacho en 1–3 días hábiles en Bogotá, Medellín y Cali.")
	c.MetricsAddr = strings.TrimSpace(os.Getenv("SSH_METRICS_ADDR"))
	c.DisablePasswordAuth = getenvBoolDefault("SSH_DISABLE_PASSWORD_AUTH", false)
	c.DisablePortForwarding = getenvBoolDefault("SSH_DISABLE_PORT_FORWARDING", true)
	c.CoreAPIHMACSecret = os.Getenv("CORE_API_HMAC_SECRET")
	c.CoreAPIHMACKeyID = strings.TrimSpace(os.Getenv("CORE_API_HMAC_KEY_ID"))
	c.CoreAPIHMACKeys = parseKeyRing(os.Getenv("CORE_API_HMAC_KEYS"))

	if _, err := url.ParseRequestURI(c.CoreAPIURL); err != nil {
		return c, fmt.Errorf("CORE_API_URL invalid: %w", err)
	}
	if c.RequireAuthorizedKeys && c.AuthorizedKeysPath == "" {
		return c, fmt.Errorf("SSH_AUTHORIZED_KEYS_PATH is required when SSH_REQUIRE_AUTHORIZED_KEYS=1")
	}
	if c.RequireAuthorizedKeys && c.DevFingerprint != "" {
		return c, fmt.Errorf("SSH_DEV_FINGERPRINT cannot be used with SSH_REQUIRE_AUTHORIZED_KEYS")
	}
	return c, nil
}

// parseKeyRing parses "kid1:secret1,kid2:secret2" into a map.
func parseKeyRing(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kid, sec, ok := strings.Cut(part, ":")
		kid = strings.TrimSpace(kid)
		sec = strings.TrimSpace(sec)
		if !ok || kid == "" || sec == "" {
			continue
		}
		out[kid] = sec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getenvDefault(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func getenvIntDefault(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvBoolDefault(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}
