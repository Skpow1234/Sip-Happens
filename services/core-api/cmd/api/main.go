package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"sip-happens/pkg/config"
	"sip-happens/pkg/healthcheck"
	"sip-happens/pkg/redisutil"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	_ "sip-happens/services/core-api/internal/consumers"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/email"
	httpserver "sip-happens/services/core-api/internal/http"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/objectstore"
	"sip-happens/services/core-api/internal/observability"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/seed"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "healthcheck" {
		if err := runHealthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "seed" && os.Args[2] == "demo" {
		if err := runSeedDemo(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := runServer(); err != nil {
		log.Fatal(err)
	}
}

func runServer() error {
	ctx := context.Background()

	cfg, err := config.LoadCoreAPI()
	if err != nil {
		return err
	}

	sqlDB, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(cfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DBPool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.DBPool.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.DBPool.PingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return err
	}
	observability.RegisterDBStatsCollector(sqlDB)

	d := db.New(sqlDB)

	ordersRepo := orders.NewSQLRepository()
	invRepo := inventory.NewSQLRepository()
	catalogRepo := catalog.NewSQLRepository()
	paymentsRepo := payments.NewSQLRepository()

	ordersSvc := orders.NewServiceWithOutbox(d, ordersRepo, invRepo, catalogRepo, paymentsRepo, outbox.NewSQLStore())
	customersSvc := customers.NewService(sqlDB, customers.NewSQLRepository())
	adminSvc := &admin.SQLService{DB: sqlDB}

	stripeSigningSecret := cfg.StripeWebhookSecret
	var stripeWebhook *stripepayments.WebhookHandler
	if stripeSigningSecret != "" {
		stripeWebhook = &stripepayments.WebhookHandler{
			Verifier:   stripepayments.StripeGoVerifier{SigningSecret: stripeSigningSecret, ToleranceSec: 300},
			EventStore: &stripepayments.SQLEventStore{DB: sqlDB},
			EventLog:   &stripepayments.SQLEventLogStore{DB: sqlDB},
			Orders:     ordersSvc,
			OrderRepo:  ordersRepo,
			OrderQuery: sqlDB,
		}
	}

	var redisClient *redis.Client
	if cfg.RedisURL != "" {
		redisClient, err = redisutil.Open(cfg.RedisURL)
		if err != nil {
			return err
		}
		defer redisClient.Close()
	}

	mailer, err := email.NewMailerFromEnv()
	if err != nil {
		return err
	}
	emailSvc, err := email.NewService(mailer, sqlDB, cfg.Flags)
	if err != nil {
		return err
	}

	productImages, err := objectstore.NewProductImagesFromEnv()
	if err != nil {
		return err
	}

	srv := &httpserver.Server{
		Orders:            ordersSvc,
		Customers:         customersSvc,
		Catalog:           catalogRepo,
		Admin:             adminSvc,
		AdminToken:        cfg.AdminToken,
		AdminTokens:       cfg.AdminTokens,
		AdminTokenRoles:   httpserver.AdminTokenRolesFromConfig(cfg),
		DB:                sqlDB,
		Namespace:         cfg.Namespace,
		FeatureFlags:      cfg.Flags.Enabled,
		CoreAPIHMACSecret: cfg.CoreAPIHMACSecret,
		CoreAPIHMACKeys:   cfg.CoreAPIHMACKeys,
		Stripe:            selectStripeClient(cfg),
		StripeWebhook:     stripeWebhook,
		StripeWebhookPath: cfg.StripeWebhookPath,
		Email:             emailSvc,
		Redis:             redisClient,
		RedisRequired:     cfg.RedisRequired,
		ProductImages:     productImages,
	}

	httpSrv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: srv.Handler(),
	}
	log.Printf("core-api listening on %s", cfg.HTTPAddr)
	return httpSrv.ListenAndServe()
}

func selectStripeClient(cfg config.CoreAPI) stripepayments.Service {
	if key := cfg.StripeSecretKey; key != "" {
		return stripepayments.StripeGoClient{SecretKey: key}
	}
	if cfg.StripeMock {
		return stripepayments.MockClient{}
	}
	return nil
}

func runSeedDemo(ctx context.Context) error {
	cfg, err := config.LoadCoreAPI()
	if err != nil {
		return err
	}
	sqlDB, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	out, err := seed.Demo(ctx, sqlDB)
	if err != nil {
		return err
	}

	log.Printf("seeded demo data:")
	log.Printf("  customer_id=%s", out.CustomerID)
	log.Printf("  ssh_fingerprint=%s", out.SSHFingerprint)
	if out.DemoSSHPubKey != "" {
		log.Printf("  demo_ssh_pubkey=%s", out.DemoSSHPubKey)
		log.Printf("  tui: ssh -i infra/demo/sip-happens-demo -p 2222 localhost")
	}
	for i, pid := range out.ProductIDs {
		log.Printf("  product_id[%d]=%s", i, pid)
	}
	return nil
}

func runHealthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	url := healthcheck.LocalHTTPAddr(addr) + "/healthz"
	ctx, cancel := context.WithTimeout(context.Background(), healthcheck.ProbeTimeout)
	defer cancel()
	return healthcheck.HTTPGetOK(ctx, url)
}
