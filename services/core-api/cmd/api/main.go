package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"

	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	httpserver "sip-happens/services/core-api/internal/http"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/migrate"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/seed"
)

func main() {
	if len(os.Args) >= 3 && os.Args[1] == "migrate" && os.Args[2] == "up" {
		if err := runMigrateUp(context.Background()); err != nil {
			log.Fatal(err)
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

func runMigrateUp(ctx context.Context) error {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("POSTGRES_DSN is required")
	}
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	// Container-first note: images must include `infra/migrations` (copied into the image).
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = filepath.FromSlash("infra/migrations")
	}

	m := migrate.Migrator{
		DB:  sqlDB,
		FS:  os.DirFS(migrationsPath),
		Dir: ".",
	}
	return m.Up(ctx)
}

func runServer() error {
	ctx := context.Background()

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("POSTGRES_DSN is required")
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	d := db.New(sqlDB)

	ordersRepo := orders.NewSQLRepository()
	invRepo := inventory.NewSQLRepository()
	catalogRepo := catalog.NewSQLRepository()
	paymentsRepo := payments.NewSQLRepository()

	ordersSvc := orders.NewService(d, ordersRepo, invRepo, catalogRepo, paymentsRepo)
	adminSvc := &admin.SQLService{DB: sqlDB}

	// Stripe webhook wiring (Stripe verifier + SQL idempotency store).
	stripeSigningSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	var stripeWebhook *stripepayments.WebhookHandler
	if stripeSigningSecret != "" {
		stripeWebhook = &stripepayments.WebhookHandler{
			Verifier:   stripepayments.StripeGoVerifier{SigningSecret: stripeSigningSecret, ToleranceSec: 300},
			EventStore: &stripepayments.SQLEventStore{DB: sqlDB},
			Orders:     ordersSvc,
		}
	}

	srv := &httpserver.Server{
		Orders:        ordersSvc,
		Catalog:       catalogRepo,
		Admin:         adminSvc,
		AdminToken:    os.Getenv("ADMIN_TOKEN"),
		DB:            sqlDB,
		Stripe:        selectStripeClient(),
		StripeWebhook: stripeWebhook,
	}

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}
	log.Printf("core-api listening on %s", addr)
	return httpSrv.ListenAndServe()
}

func selectStripeClient() stripepayments.Service {
	if key := os.Getenv("STRIPE_SECRET_KEY"); key != "" {
		return stripepayments.StripeGoClient{SecretKey: key}
	}
	if os.Getenv("STRIPE_MOCK") == "1" {
		return stripepayments.MockClient{}
	}
	return nil
}

func runSeedDemo(ctx context.Context) error {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return fmt.Errorf("POSTGRES_DSN is required")
	}
	sqlDB, err := sql.Open("postgres", dsn)
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
	for i, pid := range out.ProductIDs {
		log.Printf("  product_id[%d]=%s", i, pid)
	}
	return nil
}

