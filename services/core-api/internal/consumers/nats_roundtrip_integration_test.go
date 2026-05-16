package consumers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/events"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/fulfillment"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/payments"
	"sip-happens/services/core-api/internal/testutil"
	"sip-happens/services/core-api/internal/workflows"
)

// Integration: outbox row -> JetStream publish -> order.paid consumer -> shipment + FULFILLING.
func TestOutboxPublishConsume_OrderPaid_Roundtrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("skipping (need POSTGRES_DSN or Docker): %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	ns := runJetStreamTestServer(t)
	defer ns.Shutdown()

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}

	namespace := "test"
	stream := "sip_happens_test"
	prefix := events.EventsSubjectPrefix(namespace)

	productID, customerID := seedRoundtripFixture(t, ctx, tdb)

	ordersSvc := orders.NewServiceWithOutbox(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
		outbox.NewSQLStore(),
		nil,
	)

	o, _, err := ordersSvc.CreatePendingOrder(ctx, orders.CreatePendingOrderInput{
		CustomerID:   domain.CustomerID(customerID),
		Currency:     "COP",
		City:         "Bogotá",
		Neighborhood: "Chapinero",
		AddressLine1: "Calle 1",
		Phone:        "300",
		LegalID:      "1",
		FullName:     "Roundtrip",
		Items:        []orders.CreatePendingOrderItem{{ProductID: domain.ProductID(productID), Qty: 1}},
	})
	if err != nil {
		t.Fatalf("CreatePendingOrder: %v", err)
	}

	payment := domain.Payment{Provider: domain.PaymentProviderStripe, ProviderRef: "pi_roundtrip", Status: domain.PaymentStatusSucceeded}
	if err := ordersSvc.MarkPaid(ctx, o.ID, payment); err != nil {
		t.Fatalf("MarkPaid: %v", err)
	}

	if _, err := js.AddStream(&nats.StreamConfig{
		Name:      stream,
		Subjects:  []string{events.EventsFilter(namespace)},
		Storage:   nats.MemoryStorage,
		Retention: nats.LimitsPolicy,
	}); err != nil && err != nats.ErrStreamNameAlreadyInUse {
		t.Fatalf("add stream: %v", err)
	}

	var pending int
	if err := tdb.DB.QueryRowContext(ctx, `select count(*) from outbox_events where status='PENDING'`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending == 0 {
		t.Fatal("expected pending outbox events after MarkPaid")
	}

	pub := &outbox.JetStreamPublisher{JS: js, Stream: stream, SubjectPrefix: prefix}
	disp := &outbox.Dispatcher{Store: outbox.NewSQLStore(), Publisher: pub, LockedBy: "test-dispatch", Batch: 10}

	sent, err := disp.RunOnce(ctx, tdb.DB, time.Now())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if sent == 0 {
		var lastErr sql.NullString
		_ = tdb.DB.QueryRowContext(ctx, `select coalesce(last_error,'') from outbox_events order by updated_at desc limit 1`).Scan(&lastErr)
		t.Fatalf("expected outbox dispatch to send events (pending was %d, last_error=%q)", pending, lastErr.String)
	}

	fulfillmentSvc := &fulfillment.SQLService{DB: tdb.DB}
	handler := OrderPaidHandler{
		Namespace: namespace,
		Name:      "test-consumer",
		Saga:      workflows.Saga{Fulfillment: fulfillmentSvc},
	}
	runtime := &JetStreamRuntime{
		JS:            js,
		Stream:        stream,
		Namespace:     namespace,
		FilterSubject: events.EventsFilter(namespace),
		Durable:       "test-durable-roundtrip",
		Idempotency:   &SQLIdempotencyStore{DB: tdb.DB},
		DLQSubject:    "", // avoid DLQ stream requirement in unit roundtrip
	}

	sub, err := js.PullSubscribe(events.EventsFilter(namespace), "test-durable-roundtrip", nats.BindStream(stream), nats.ManualAck())
	if err != nil {
		t.Fatalf("pull subscribe: %v", err)
	}
	msgs, err := sub.Fetch(10, nats.MaxWait(3*time.Second))
	if err != nil && err != nats.ErrTimeout {
		t.Fatalf("fetch: %v", err)
	}
	for _, msg := range msgs {
		if err := runtime.processOne(ctx, handler, msg); err != nil {
			t.Fatalf("process: %v", err)
		}
	}

	var orderStatus string
	if err := tdb.DB.QueryRowContext(ctx, `select status from orders where id=$1`, string(o.ID)).Scan(&orderStatus); err != nil {
		t.Fatal(err)
	}
	if orderStatus != string(domain.OrderStatusFulfilling) {
		t.Fatalf("expected FULFILLING, got %s", orderStatus)
	}

	var shipStatus string
	if err := tdb.DB.QueryRowContext(ctx, `select status from shipments where order_id=$1`, string(o.ID)).Scan(&shipStatus); err != nil {
		t.Fatal(err)
	}
	if shipStatus != string(domain.ShipmentStatusNew) {
		t.Fatalf("expected shipment NEW, got %s", shipStatus)
	}
}

func runJetStreamTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{Port: -1, JetStream: true, StoreDir: t.TempDir()}
	return test.RunServer(opts)
}

func seedRoundtripFixture(t *testing.T, ctx context.Context, tdb *testutil.TestDB) (productID, customerID string) {
	t.Helper()
	_, err := tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust_roundtrip','fp:rt',now(),now())
on conflict (id) do nothing
`)
	if err != nil {
		t.Fatal(err)
	}
	customerID = "cust_roundtrip"
	productID = "prod_roundtrip"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-RT','RT Beer','Brew','Lager',5.0,330,'INVIMA-RT',true,now(),now())
on conflict (id) do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tdb.DB.ExecContext(ctx, `insert into prices (product_id, currency, amount_minor_units, valid_from) values ($1,'COP',1000,now()) on conflict do nothing`, productID)
	_, _ = tdb.DB.ExecContext(ctx, `insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at) values ($1,10,0,0,now()) on conflict (product_id) do update set on_hand=10, reserved=0`, productID)
	return productID, customerID
}
