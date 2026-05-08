package main

import (
	"context"
	"database/sql"
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
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/consumers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/fulfillment"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: core-jobs <cancel-expired|stripe-reconcile|outbox-dispatch|outbox-replay|nats-consume|inventory-reconcile|fulfillment-runner>")
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

	publisher, cleanup, err := outboxPublisherFromEnv()
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

	log.Printf("job outbox-dispatch starting (interval=%s, limit=%d, once=%v, locked_by=%s)", jobCfg.Interval, jobCfg.BatchLimit, jobCfg.RunOnce, jobCfg.LockedBy)

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

type logConsumer struct {
	name string
}

func (c logConsumer) ConsumerName() string { return c.name }

func (c logConsumer) Handle(ctx context.Context, m *nats.Msg) error {
	_ = ctx
	log.Printf("consume subject=%s event_id=%s event_type=%s request_id=%s", m.Subject, m.Header.Get(outbox.NatsHeaderEventID), m.Header.Get(outbox.NatsHeaderEventType), m.Header.Get(outbox.NatsHeaderRequestID))
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
		filter = fmt.Sprintf("sip_happens.%s.events.>", namespace)
	}
	durable := os.Getenv("NATS_DURABLE")
	if durable == "" {
		durable = "core-api"
	}
	consumerName := os.Getenv("NATS_CONSUMER_NAME")
	if consumerName == "" {
		consumerName = "core-api"
	}
	dlq := os.Getenv("NATS_DLQ_SUBJECT")
	if dlq == "" {
		dlq = fmt.Sprintf("sip_happens.%s.dlq", namespace)
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

	r := &consumers.JetStreamRuntime{
		JS:            js,
		Stream:        stream,
		FilterSubject: filter,
		Durable:       durable,
		Idempotency:   &consumers.SQLIdempotencyStore{DB: sqlDB},
		DLQSubject:    dlq,
	}

	return r.Run(ctx, logConsumer{name: consumerName})
}

func outboxPublisherFromEnv() (outbox.Publisher, func(), error) {
	kind := os.Getenv("OUTBOX_PUBLISHER")
	if kind == "" || kind == "log" {
		return outbox.LogPublisher{}, func() {}, nil
	}
	if kind != "nats" {
		return nil, nil, fmt.Errorf("unknown OUTBOX_PUBLISHER=%q (expected 'log' or 'nats')", kind)
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return nil, nil, fmt.Errorf("NATS_URL is required when OUTBOX_PUBLISHER=nats")
	}
	stream := os.Getenv("NATS_STREAM")
	if stream == "" {
		return nil, nil, fmt.Errorf("NATS_STREAM is required when OUTBOX_PUBLISHER=nats")
	}
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "dev"
	}
	subjectPrefix := os.Getenv("NATS_SUBJECT_PREFIX")
	if subjectPrefix == "" {
		subjectPrefix = fmt.Sprintf("sip_happens.%s.events", namespace)
	}

	nc, err := nats.Connect(natsURL, nats.Name("sip-happens outbox-dispatch"))
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}

	pub := &outbox.JetStreamPublisher{
		JS:            js,
		Stream:        stream,
		SubjectPrefix: subjectPrefix,
	}
	return pub, func() { nc.Close() }, nil
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
