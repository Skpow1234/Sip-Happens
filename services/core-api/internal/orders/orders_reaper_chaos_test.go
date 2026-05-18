package orders

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/payments"
	"sip-happens/services/core-api/internal/testutil"
)

// Chaos: order-reaper (cancel-expired job) racing with MarkPaid on the same expired order.
func TestChaos_ReaperVsMarkPaid_ExclusiveOutcome(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	const rounds = 25
	fix := seedChaosProduct(ctx, t, tdb.DB, "CHAOS-REAPER-PAID", rounds+5)
	svc := newChaosService(tdb.DB)
	for round := 0; round < rounds; round++ {
		o, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
			CustomerID: fix.customerID,
			Currency:   "COP",
			City:       "Bogotá",
			Items:      []CreatePendingOrderItem{{ProductID: fix.productID, Qty: 1}},
		})
		if err != nil {
			t.Fatalf("round %d create: %v", round, err)
		}
		expired := time.Now().Add(-time.Minute)
		if _, err := tdb.DB.ExecContext(ctx, `update orders set expires_at=$1 where id=$2`, expired, string(o.ID)); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		var reaperErr, paidErr error
		go func() {
			defer wg.Done()
			_, reaperErr = svc.CancelExpiredPendingOrders(ctx, time.Now(), 10)
		}()
		go func() {
			defer wg.Done()
			ref, err := ids.UUIDv4()
			if err != nil {
				paidErr = err
				return
			}
			paidErr = svc.MarkPaid(ctx, o.ID, domain.Payment{
				Provider:    domain.PaymentProviderStripe,
				ProviderRef: ref,
				Status:      domain.PaymentStatusSucceeded,
				RawPayload:  `{"chaos":"reaper-vs-paid"}`,
			})
		}()
		wg.Wait()
		if reaperErr != nil {
			t.Fatalf("round %d reaper: %v", round, reaperErr)
		}

		st, onHand, reserved := loadOrderAndInventory(ctx, t, tdb.DB, o.ID, fix.productID)
		if st != domain.OrderStatusPaid && st != domain.OrderStatusCancelled {
			t.Fatalf("round %d: status=%s want PAID or CANCELLED (paidErr=%v)", round, st, paidErr)
		}
		if onHand < 0 || reserved < 0 {
			t.Fatalf("round %d: negative inventory on_hand=%d reserved=%d", round, onHand, reserved)
		}
		if st == domain.OrderStatusPaid {
			if paidErr != nil && !errors.Is(paidErr, domain.ErrAlreadyApplied) {
				t.Fatalf("round %d: paid but MarkPaid err=%v", round, paidErr)
			}
		} else if paidErr == nil {
			t.Fatalf("round %d: cancelled but MarkPaid succeeded", round)
		}

		// Replenish stock so the next round can reserve again.
		if _, err := tdb.DB.ExecContext(ctx, `
update inventory set on_hand=$1, reserved=0, updated_at=now() where product_id=$2
`, rounds+5, string(fix.productID)); err != nil {
			t.Fatal(err)
		}
	}
}

// Chaos: cancel-expired job vs a new checkout on stock freed by an expired reservation.
func TestChaos_ReaperVsCheckout_NoOversell(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	fix := seedChaosProduct(ctx, t, tdb.DB, "CHAOS-REAPER-CO", 1)
	otherCustomer, _ := ids.UUIDv4()
	now := time.Now()
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$4) on conflict do nothing
`, otherCustomer, "chaos:other", now, now)
	if err != nil {
		t.Fatal(err)
	}

	svc := newChaosService(tdb.DB)
	expiredOrder, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
		CustomerID: fix.customerID,
		Currency:   "COP",
		City:       "Bogotá",
		Items:      []CreatePendingOrderItem{{ProductID: fix.productID, Qty: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-2 * time.Minute)
	if _, err := tdb.DB.ExecContext(ctx, `update orders set expires_at=$1 where id=$2`, expired, string(expiredOrder.ID)); err != nil {
		t.Fatal(err)
	}

	const attempts = 20
	var wg sync.WaitGroup
	wg.Add(attempts * 2)

	var mu sync.Mutex
	checkoutOK := 0

	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.CancelExpiredPendingOrders(ctx, time.Now(), 5)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("reaper: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
				CustomerID: domain.CustomerID(otherCustomer),
				Currency:   "COP",
				City:       "Bogotá",
				Items:      []CreatePendingOrderItem{{ProductID: fix.productID, Qty: 1}},
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				checkoutOK++
				return
			}
			if errors.Is(err, domain.ErrConflict) {
				return
			}
			t.Errorf("checkout: %v", err)
		}()
	}
	wg.Wait()

	_, onHand, reserved := loadOrderAndInventory(ctx, t, tdb.DB, expiredOrder.ID, fix.productID)
	if onHand < 0 || reserved < 0 {
		t.Fatalf("negative inventory on_hand=%d reserved=%d", onHand, reserved)
	}
	if onHand-reserved < 0 {
		t.Fatalf("reserved exceeds on_hand: on_hand=%d reserved=%d", onHand, reserved)
	}
	if checkoutOK > 1 {
		t.Fatalf("oversold: %d successful checkouts on 1 on_hand", checkoutOK)
	}
}

type chaosFixture struct {
	customerID domain.CustomerID
	productID  domain.ProductID
}

func newChaosService(sqlDB *sql.DB) Service {
	return NewService(
		db.New(sqlDB),
		NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)
}

func seedChaosProduct(ctx context.Context, t *testing.T, sqlDB *sql.DB, sku string, onHand int) chaosFixture {
	t.Helper()
	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	now := time.Now()

	_, err := sqlDB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$4) on conflict do nothing
`, customerID, "chaos:"+sku, now, now)
	if err != nil {
		t.Fatal(err)
	}
	err = sqlDB.QueryRowContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,$3,'Chaos','IPA',5.0,330,'INVIMA-CHAOS',true,$4,$4)
on conflict (sku) do update set is_active=true, updated_at=excluded.updated_at
returning id
`, productID, sku, "Chaos "+sku, now).Scan(&productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',1000,now()) on conflict do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,$2,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0, safety_stock=0, updated_at=now()
`, productID, onHand)
	if err != nil {
		t.Fatal(err)
	}
	return chaosFixture{
		customerID: domain.CustomerID(customerID),
		productID:  domain.ProductID(productID),
	}
}

func loadOrderAndInventory(ctx context.Context, t *testing.T, sqlDB *sql.DB, orderID domain.OrderID, productID domain.ProductID) (domain.OrderStatus, int, int) {
	t.Helper()
	var st string
	if err := sqlDB.QueryRowContext(ctx, `select status from orders where id=$1`, string(orderID)).Scan(&st); err != nil {
		t.Fatal(err)
	}
	var onHand, reserved int
	if err := sqlDB.QueryRowContext(ctx, `select on_hand, reserved from inventory where product_id=$1`, string(productID)).Scan(&onHand, &reserved); err != nil {
		t.Fatal(err)
	}
	return domain.OrderStatus(st), onHand, reserved
}
