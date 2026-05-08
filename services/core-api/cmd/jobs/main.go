package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"sip-happens/pkg/config"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: core-jobs <cancel-expired|stripe-reconcile|outbox-dispatch|outbox-replay|inventory-reconcile>")
	}
	cmd := os.Args[1]

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "cancel-expired":
		if err := runCancelExpiredJob(ctx); err != nil {
			log.Fatal(err)
		}
	case "stripe-reconcile":
		if err := runStripeReconcileJob(ctx); err != nil {
			log.Fatal(err)
		}
	case "outbox-dispatch":
		if err := runOutboxDispatchJob(ctx); err != nil {
			log.Fatal(err)
		}
	case "outbox-replay":
		if err := runOutboxReplayJob(ctx); err != nil {
			log.Fatal(err)
		}
	case "inventory-reconcile":
		if err := runInventoryReconcileJob(ctx); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}

func openDB(ctx context.Context, dsn string) (*sql.DB, error) {
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

func runCancelExpiredJob(ctx context.Context) error {
	jobCfg, err := config.LoadCancelExpiredJob()
	if err != nil {
		return err
	}
	pingCtx, cancel := context.WithTimeout(ctx, jobCfg.DBPool.PingTimeout)
	defer cancel()
	sqlDB, err := openDB(pingCtx, jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(jobCfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(jobCfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(jobCfg.DBPool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(jobCfg.DBPool.ConnMaxIdleTime)

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
	pingCtx, cancel := context.WithTimeout(ctx, jobCfg.DBPool.PingTimeout)
	defer cancel()
	sqlDB, err := openDB(pingCtx, jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(jobCfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(jobCfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(jobCfg.DBPool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(jobCfg.DBPool.ConnMaxIdleTime)

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
	pingCtx, cancel := context.WithTimeout(ctx, jobCfg.DBPool.PingTimeout)
	defer cancel()
	sqlDB, err := openDB(pingCtx, jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(jobCfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(jobCfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(jobCfg.DBPool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(jobCfg.DBPool.ConnMaxIdleTime)

	d := &outbox.Dispatcher{
		Store:       outbox.NewSQLStore(),
		Publisher:   outbox.LogPublisher{},
		LockedBy:    jobCfg.LockedBy,
		Batch:       jobCfg.BatchLimit,
		MaxAttempts: jobCfg.MaxAttempts,
		BaseBackoff: jobCfg.BaseBackoff,
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
	pingCtx, cancel := context.WithTimeout(ctx, jobCfg.DBPool.PingTimeout)
	defer cancel()
	sqlDB, err := openDB(pingCtx, jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(jobCfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(jobCfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(jobCfg.DBPool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(jobCfg.DBPool.ConnMaxIdleTime)

	store := outbox.NewSQLStore()
	log.Printf("job outbox-replay starting (limit=%d)", jobCfg.Limit)
	n, err := store.ReplayDeadLetters(ctx, sqlDB, jobCfg.Limit)
	if err != nil {
		return err
	}
	log.Printf("job outbox-replay: requeued=%d", n)
	return nil
}

func runInventoryReconcileJob(ctx context.Context) error {
	jobCfg, err := config.LoadInventoryReconcileJob()
	if err != nil {
		return err
	}
	pingCtx, cancel := context.WithTimeout(ctx, jobCfg.DBPool.PingTimeout)
	defer cancel()
	sqlDB, err := openDB(pingCtx, jobCfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(jobCfg.DBPool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(jobCfg.DBPool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(jobCfg.DBPool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(jobCfg.DBPool.ConnMaxIdleTime)

	job := &inventory.ReconcileJob{
		DB:          sqlDB,
		Inventory:   inventory.NewSQLRepository(),
		BatchLimit:  jobCfg.BatchLimit,
		ExpireGrace: jobCfg.ExpireGrace,
		Fix:         jobCfg.Fix,
	}

	tick := time.NewTicker(jobCfg.Interval)
	defer tick.Stop()

	log.Printf("job inventory-reconcile starting (interval=%s, limit=%d, once=%v, fix=%v, expire_grace=%s)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce, jobCfg.Fix, jobCfg.ExpireGrace)

	for {
		fixed, flagged, err := job.RunOnce(ctx, time.Now())
		if err != nil {
			return err
		}
		if flagged > 0 || fixed > 0 {
			log.Printf("job inventory-reconcile: flagged=%d fixed=%d", flagged, fixed)
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

