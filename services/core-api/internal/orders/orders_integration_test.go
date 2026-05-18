package orders

import (
	"context"
	"errors"
	"fmt"
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

// Integration test: concurrent reservations must not oversell limited stock.
func TestConcurrentCreatePendingOrder_DoesNotOversell(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	sku := fmt.Sprintf("ITEST-SKU-%s", productID)
	now := time.Now()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$4)
on conflict do nothing
`, customerID, "itest:ssh", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,true,$9,$10)
`, productID, sku, "ITest Beer", "ITest", "IPA", 5.0, 330, "INVIMA-ITEST", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',1000,now())
on conflict do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,5,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0, safety_stock=0, updated_at=now()
`, productID)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(
		db.New(tdb.DB),
		NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)

	const attempts = 10
	var wg sync.WaitGroup
	wg.Add(attempts)

	var mu sync.Mutex
	success := 0
	conflicts := 0

	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			_, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
				CustomerID: domain.CustomerID(customerID),
				Currency:   "COP",
				City:       "Bogotá",
				Items: []CreatePendingOrderItem{
					{ProductID: domain.ProductID(productID), Qty: 1},
				},
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success++
				return
			}
			if errors.Is(err, domain.ErrConflict) {
				conflicts++
				return
			}
			t.Errorf("unexpected error: %v", err)
		}()
	}
	wg.Wait()

	if success != 5 {
		t.Fatalf("expected 5 successes, got %d (conflicts=%d)", success, conflicts)
	}
}

func TestMarkPaid_IdempotentOnDuplicateProviderRef(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	sku := fmt.Sprintf("ITEST-SKU-PAID-%s", productID)
	now := time.Now()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$4)
on conflict do nothing
`, customerID, "itest:ssh-paid", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,true,$9,$10)
`, productID, sku, "ITest Paid Beer", "ITest", "IPA", 5.0, 330, "INVIMA-ITEST", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',1000,now())
on conflict do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,1,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0, safety_stock=0, updated_at=now()
`, productID)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(
		db.New(tdb.DB),
		NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)

	o, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
		CustomerID: domain.CustomerID(customerID),
		Currency:   "COP",
		City:       "Bogotá",
		Items: []CreatePendingOrderItem{
			{ProductID: domain.ProductID(productID), Qty: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	p := domain.Payment{
		Provider:    domain.PaymentProviderStripe,
		ProviderRef: "pi_itest_duplicate",
		Status:      domain.PaymentStatusSucceeded,
		RawPayload:  `{"itest":true}`,
	}

	if err := svc.MarkPaid(ctx, o.ID, p); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkPaid(ctx, o.ID, p); err != nil && err != domain.ErrAlreadyApplied {
		t.Fatalf("expected nil or ErrAlreadyApplied, got: %v", err)
	}
}
