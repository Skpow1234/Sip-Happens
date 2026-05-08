package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/pkg/config"
	"sip-happens/services/core-api/internal/db"
	httpserver "sip-happens/services/core-api/internal/http"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/migrate"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/outbox"
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

	if len(os.Args) >= 3 && os.Args[1] == "jobs" && os.Args[2] == "cancel-expired" {
		if err := runCancelExpiredJob(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(os.Args) >= 3 && os.Args[1] == "jobs" && os.Args[2] == "stripe-reconcile" {
		if err := runStripeReconcileJob(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(os.Args) >= 3 && os.Args[1] == "jobs" && os.Args[2] == "outbox-dispatch" {
		if err := runOutboxDispatchJob(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}

	if len(os.Args) >= 3 && os.Args[1] == "jobs" && os.Args[2] == "outbox-replay" {
		if err := runOutboxReplayJob(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := runServer(); err != nil {
		log.Fatal(err)
	}
}

func runMigrateUp(ctx context.Context) error {
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

	m := migrate.Migrator{
		DB:  sqlDB,
		FS:  os.DirFS(cfg.MigrationsPath),
		Dir: ".",
	}
	return m.Up(ctx)
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

	srv := &httpserver.Server{
		Orders:        ordersSvc,
		Catalog:       catalogRepo,
		Admin:         adminSvc,
		AdminToken:    cfg.AdminToken,
		AdminTokens:   cfg.AdminTokens,
		DB:            sqlDB,
		CoreAPIHMACSecret: cfg.CoreAPIHMACSecret,
		CoreAPIHMACKeys:   cfg.CoreAPIHMACKeys,
		Stripe:        selectStripeClient(cfg),
		StripeWebhook: stripeWebhook,
		StripeWebhookPath: cfg.StripeWebhookPath,
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
	for i, pid := range out.ProductIDs {
		log.Printf("  product_id[%d]=%s", i, pid)
	}
	return nil
}

func runCancelExpiredJob(ctx context.Context) error {
	jobCfg, err := config.LoadCancelExpiredJob()
	if err != nil {
		return err
	}
	sqlDB, err := sql.Open("postgres", jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	d := db.New(sqlDB)
	ordersRepo := orders.NewSQLRepository()
	invRepo := inventory.NewSQLRepository()
	catalogRepo := catalog.NewSQLRepository()
	paymentsRepo := payments.NewSQLRepository()
	ordersSvc := orders.NewService(d, ordersRepo, invRepo, catalogRepo, paymentsRepo)

	tick := time.NewTicker(jobCfg.Interval)
	defer tick.Stop()

	log.Printf("job cancel-expired starting (interval=%s, limit=%d, once=%v)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce)

	for {
		n, err := ordersSvc.CancelExpiredPendingOrders(ctx, time.Now(), jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("job cancel-expired: cancelled=%d", n)
		}

		if jobCfg.RunOnce {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func runStripeReconcileJob(ctx context.Context) error {
	jobCfg, err := config.LoadStripeReconcileJob()
	if err != nil {
		return err
	}
	sqlDB, err := sql.Open("postgres", jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	d := db.New(sqlDB)
	ordersRepo := orders.NewSQLRepository()
	invRepo := inventory.NewSQLRepository()
	catalogRepo := catalog.NewSQLRepository()
	paymentsRepo := payments.NewSQLRepository()
	ordersSvc := orders.NewService(d, ordersRepo, invRepo, catalogRepo, paymentsRepo)

	var stripeClient stripepayments.Service
	if jobCfg.StripeSecretKey != "" {
		stripeClient = stripepayments.StripeGoClient{SecretKey: jobCfg.StripeSecretKey}
	} else if jobCfg.StripeMock {
		stripeClient = stripepayments.MockClient{}
	}
	if stripeClient == nil {
		return nil
	}

	job := &stripepayments.ReconcileJob{
		DB:        sqlDB,
		Orders:    ordersSvc,
		OrderRepo: ordersRepo,
		Stripe:    stripeClient,
		Log:       &stripepayments.SQLEventLogStore{DB: sqlDB},
		Queue:     &stripepayments.SQLReconcileQueue{DB: sqlDB},
	}

	tick := time.NewTicker(jobCfg.Interval)
	defer tick.Stop()

	log.Printf("job stripe-reconcile starting (interval=%s, limit=%d, once=%v)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce)

	for {
		_, err := job.EnqueueStalePending(ctx, time.Now(), 2*time.Minute, jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		n, err := job.RunBatch(ctx, time.Now(), jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("job stripe-reconcile: processed=%d", n)
		}

		if jobCfg.RunOnce {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func runOutboxDispatchJob(ctx context.Context) error {
	jobCfg, err := config.LoadOutboxDispatchJob()
	if err != nil {
		return err
	}
	sqlDB, err := sql.Open("postgres", jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	d := &outbox.Dispatcher{
		Store:     outbox.NewSQLStore(),
		Publisher: outbox.LogPublisher{},
		LockedBy:  jobCfg.LockedBy,
		Batch:     jobCfg.BatchLimit,
	}

	tick := time.NewTicker(jobCfg.Interval)
	defer tick.Stop()

	log.Printf("job outbox-dispatch starting (interval=%s, limit=%d, once=%v, locked_by=%s)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce, jobCfg.LockedBy)

	for {
		n, err := d.RunOnce(ctx, sqlDB, time.Now())
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("job outbox-dispatch: sent=%d", n)
		}

		if jobCfg.RunOnce {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func runOutboxReplayJob(ctx context.Context) error {
	jobCfg, err := config.LoadOutboxReplayJob()
	if err != nil {
		return err
	}
	sqlDB, err := sql.Open("postgres", jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	store := outbox.NewSQLStore()

	log.Printf("job outbox-replay starting (limit=%d)", jobCfg.Limit)
	n, err := store.ReplayDeadLetters(ctx, sqlDB, jobCfg.Limit)
	if err != nil {
		return err
	}
	log.Printf("job outbox-replay: requeued=%d", n)
	return nil
}

