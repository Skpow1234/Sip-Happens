package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"

	"sip-happens/pkg/config"
	"sip-happens/pkg/events"
	"sip-happens/pkg/featureflags"
	"sip-happens/services/core-api/internal/audit"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/consumers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/email"
	"sip-happens/services/core-api/internal/emailsubscribe"
	"sip-happens/services/core-api/internal/fulfillment"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/invoicing"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/workflows"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: core-jobs <cancel-expired|stripe-reconcile|outbox-dispatch|outbox-replay|nats-consume|inventory-reconcile|fulfillment-runner|email-subscribe-reap>")
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
	case "nats-consume":
		if err := runNATSConsumeJob(ctx); err != nil {
			log.Fatal(err)
		}
	case "inventory-reconcile":
		if err := runInventoryReconcileJob(ctx); err != nil {
			log.Fatal(err)
		}
	case "fulfillment-runner":
		if err := runFulfillmentRunner(ctx); err != nil {
			log.Fatal(err)
		}
	case "email-subscribe-reap":
		if err := runEmailSubscribeReapJob(ctx); err != nil {
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
	auditStore := audit.NewSQLStore()
	ordersSvc := orders.NewServiceWithOutbox(d, ordersRepo, invRepo, catalogRepo, paymentsRepo, outbox.NewSQLStore(), auditStore)

	log.Printf("job cancel-expired starting (interval=%s, limit=%d, once=%v)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce)

	idle := 0
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
		idle = updateIdle(idle, n > 0)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextPollDelay(jobCfg.Interval, idle)):
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
	auditStore := audit.NewSQLStore()
	ordersSvc := orders.NewServiceWithOutbox(d, ordersRepo, invRepo, catalogRepo, paymentsRepo, outbox.NewSQLStore(), auditStore)

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
		Payments:  paymentsRepo,
		Stripe:    stripeClient,
		Log:       &stripepayments.SQLEventLogStore{DB: sqlDB},
		Queue:     &stripepayments.SQLReconcileQueue{DB: sqlDB},
	}

	log.Printf("job stripe-reconcile starting (interval=%s, limit=%d, once=%v)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce)

	idle := 0
	for {
		enq, err := job.EnqueueStalePending(ctx, time.Now(), 2*time.Minute, jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		n, err := job.RunBatch(ctx, time.Now(), jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		work := n > 0 || enq > 0
		if n > 0 {
			log.Printf("job stripe-reconcile: processed=%d", n)
		}
		if jobCfg.RunOnce {
			return nil
		}
		idle = updateIdle(idle, work)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextPollDelay(jobCfg.Interval, idle)):
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

	pubCfg, err := config.LoadOutboxPublisher()
	if err != nil {
		return err
	}
	publisher, cleanup, err := outbox.PublisherFromEnv()
	if err != nil {
		return err
	}
	defer cleanup()

	d := &outbox.Dispatcher{
		Store:       outbox.NewSQLStore(),
		Publisher:   publisher,
		LockedBy:    jobCfg.LockedBy,
		Batch:       jobCfg.BatchLimit,
		MaxAttempts: jobCfg.MaxAttempts,
		BaseBackoff: jobCfg.BaseBackoff,
	}

	log.Printf("job outbox-dispatch starting (publisher=%s, interval=%s, limit=%d, once=%v, locked_by=%s)",
		pubCfg.Kind, jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce, jobCfg.LockedBy)

	idle := 0
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
		idle = updateIdle(idle, n > 0)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextPollDelay(jobCfg.Interval, idle)):
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

func runNATSConsumeJob(ctx context.Context) error {
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

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return fmt.Errorf("NATS_URL is required")
	}
	stream := os.Getenv("NATS_STREAM")
	if stream == "" {
		return fmt.Errorf("NATS_STREAM is required")
	}
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "dev"
	}

	filter := os.Getenv("NATS_CONSUME_SUBJECT")
	if filter == "" {
		filter = events.EventsFilter(namespace)
	} else if err := events.ValidateEventsFilter(namespace, filter); err != nil {
		return err
	}
	durable := os.Getenv("NATS_DURABLE")
	if durable == "" {
		durable = "order-paid-fulfillment"
	}
	consumerName := os.Getenv("NATS_CONSUMER_NAME")
	if consumerName == "" {
		consumerName = "order-paid-fulfillment"
	}
	dlq := os.Getenv("NATS_DLQ_SUBJECT")
	if dlq == "" {
		dlq = events.DLQSubject(namespace)
	} else if err := events.ValidateDLQSubject(namespace, dlq); err != nil {
		return err
	}

	nc, err := nats.Connect(natsURL, nats.Name("sip-happens consumer"))
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}

	// Ensure the stream exists so a fresh environment can start consumers
	// before any publisher has emitted messages.
	{
		ensureCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if _, err := js.StreamInfo(stream, nats.Context(ensureCtx)); err != nil {
			if err != nats.ErrStreamNotFound {
				return err
			}
			cfg := &nats.StreamConfig{
				Name: stream,
				Subjects: []string{
					events.EventsFilter(namespace),
					events.DLQSubject(namespace),
				},
				Storage:   nats.FileStorage,
				Retention: nats.LimitsPolicy,
			}
			if _, err := js.AddStream(cfg, nats.Context(ensureCtx)); err != nil {
				return err
			}
		}
	}

	fulfillmentSvc := &fulfillment.SQLService{DB: sqlDB}
	flags := featureflags.Parse(os.Getenv("FEATURE_FLAGS"))
	var emailSvc *email.Service
	if mailer, err := email.NewMailerFromEnv(); err != nil {
		return err
	} else {
		emailSvc, err = email.NewService(mailer, sqlDB, flags)
		if err != nil {
			return err
		}
	}
	d := db.New(sqlDB)
	ordersRepo := orders.NewSQLRepository()
	invRepo := inventory.NewSQLRepository()
	catalogRepo := catalog.NewSQLRepository()
	paymentsRepo := payments.NewSQLRepository()
	auditStore := audit.NewSQLStore()
	ordersSvc := orders.NewServiceWithOutbox(d, ordersRepo, invRepo, catalogRepo, paymentsRepo, outbox.NewSQLStore(), auditStore)
	invoiceSvc := &invoicing.Service{
		DB:       sqlDB,
		Provider: invoicing.NewProviderFromEnv(),
		Orders:   ordersSvc,
		Audit:    auditStore,
	}
	handler := consumers.OrderPaidHandler{
		Namespace: namespace,
		Name:      consumerName,
		Saga: workflows.Saga{
			Fulfillment: fulfillmentSvc,
		},
		Emails:   emailSvc,
		Invoices: invoiceSvc,
		Flags:    flags,
	}

	r := &consumers.JetStreamRuntime{
		JS:            js,
		Stream:        stream,
		Namespace:     namespace,
		FilterSubject: filter,
		Durable:       durable,
		Idempotency:   &consumers.SQLIdempotencyStore{DB: sqlDB},
		DLQSubject:    dlq,
	}

	runOnce := os.Getenv("JOB_RUN_ONCE") == "true" || os.Getenv("JOB_RUN_ONCE") == "1"
	if !runOnce {
		return r.Run(ctx, handler)
	}

	onceCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err = r.Run(onceCtx, handler)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
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

	log.Printf("job inventory-reconcile starting (interval=%s, limit=%d, once=%v, fix=%v, expire_grace=%s)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce, jobCfg.Fix, jobCfg.ExpireGrace)

	idle := 0
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
		idle = updateIdle(idle, (fixed+flagged) > 0)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextPollDelay(jobCfg.Interval, idle)):
		}
	}
}

func runFulfillmentRunner(ctx context.Context) error {
	jobCfg, err := config.LoadInventoryReconcileJob()
	if err != nil {
		return err
	}
	// Reuse JOB_* + DB_* for now; add dedicated config later if needed.
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

	svc := &fulfillment.SQLService{DB: sqlDB}
	log.Printf("job fulfillment-runner starting (interval=%s, limit=%d, once=%v)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce)
	idle := 0
	for {
		n, err := svc.RunOnce(ctx, time.Now(), jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("job fulfillment-runner: advanced=%d", n)
		}
		if jobCfg.RunOnce {
			return nil
		}
		idle = updateIdle(idle, n > 0)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextPollDelay(jobCfg.Interval, idle)):
		}
	}
}

// runEmailSubscribeReapJob clears stale email_confirmation_tokens and
// transitions PENDING_CONFIRMATION subscribers whose only token has
// expired to EXPIRED. Tunables via env:
//
//   - JOB_INTERVAL              poll interval (default 1m)
//   - JOB_BATCH_LIMIT           max rows per pass (default 500)
//   - JOB_RUN_ONCE              "true" → run a single pass and exit
//   - EMAIL_TOKEN_RETENTION     duration (Go format) for which dead
//                               tokens are retained before deletion
//                               (default 720h = 30d)
//
// Uses the same DB-pool defaults as cancel-expired.
func runEmailSubscribeReapJob(ctx context.Context) error {
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

	retention := 30 * 24 * time.Hour
	if v := os.Getenv("EMAIL_TOKEN_RETENTION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("EMAIL_TOKEN_RETENTION: %w", err)
		}
		retention = d
	}

	svc := emailsubscribe.NewService(sqlDB)
	log.Printf("job email-subscribe-reap starting (interval=%s, limit=%d, once=%v, retention=%s)",
		jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce, retention)

	idle := 0
	for {
		exp, del, err := svc.ReapExpired(ctx, time.Now(), retention, jobCfg.BatchLimit)
		if err != nil {
			return err
		}
		if exp > 0 || del > 0 {
			log.Printf("job email-subscribe-reap: subscribers_expired=%d tokens_deleted=%d", exp, del)
		}
		if jobCfg.RunOnce {
			return nil
		}
		idle = updateIdle(idle, exp > 0 || del > 0)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextPollDelay(jobCfg.Interval, idle)):
		}
	}
}

func updateIdle(idle int, didWork bool) int {
	if didWork {
		return 0
	}
	if idle < 30 {
		return idle + 1
	}
	return idle
}

func nextPollDelay(base time.Duration, idle int) time.Duration {
	if base <= 0 {
		base = 1 * time.Second
	}
	// Backpressure/cost control: if idle, exponentially back off up to 5 minutes.
	max := 5 * time.Minute
	if idle <= 0 {
		return base
	}
	f := math.Pow(2, float64(minInt(idle, 10)))
	d := time.Duration(float64(base) * f)
	if d > max {
		d = max
	}
	// Add jitter (+/- 10%) to avoid thundering herds.
	j := 0.1 * (rand.Float64()*2 - 1)
	d = time.Duration(float64(d) * (1 + j))
	if d < base {
		d = base
	}
	return d
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
