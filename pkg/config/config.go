package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type CoreAPI struct {
	HTTPAddr string

	PostgresDSN    string
	MigrationsPath string

	StripeSecretKey     string
	StripeWebhookSecret string
	StripeWebhookPath   string
	StripeMock          bool

	AdminToken string
	AdminTokens []string

	CoreAPIHMACSecret string
}

func LoadCoreAPI() (CoreAPI, error) {
	var c CoreAPI

	c.HTTPAddr = getenvDefault("HTTP_ADDR", ":8080")
	c.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if c.PostgresDSN == "" {
		return c, fmt.Errorf("POSTGRES_DSN is required")
	}

	c.MigrationsPath = getenvDefault("MIGRATIONS_PATH", "infra/migrations")

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
	c.CoreAPIHMACSecret = os.Getenv("CORE_API_HMAC_SECRET")

	return c, nil
}

type CancelExpiredJob struct {
	PostgresDSN string
	Interval    time.Duration
	BatchLimit  int
	RunOnce     bool
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
	if c.BatchLimit <= 0 {
		return c, fmt.Errorf("JOB_BATCH_LIMIT must be > 0")
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
	if c.BatchLimit <= 0 {
		return c, fmt.Errorf("JOB_BATCH_LIMIT must be > 0")
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

type SSHGateway struct {
	SSHAddr     string
	CoreAPIURL  string
	HostKeyPath string

	CoreAPIHMACSecret string
}

func LoadSSHGateway() (SSHGateway, error) {
	var c SSHGateway
	c.SSHAddr = getenvDefault("SSH_ADDR", "0.0.0.0:2222")
	c.CoreAPIURL = getenvDefault("CORE_API_URL", "http://core-api:8080")
	c.HostKeyPath = getenvDefault("SSH_HOST_KEY_PATH", "/data/ssh_host_ed25519_key")
	c.CoreAPIHMACSecret = os.Getenv("CORE_API_HMAC_SECRET")

	if _, err := url.ParseRequestURI(c.CoreAPIURL); err != nil {
		return c, fmt.Errorf("CORE_API_URL invalid: %w", err)
	}
	return c, nil
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

