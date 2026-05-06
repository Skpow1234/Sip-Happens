package orders

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/payments"
)

// Integration test (optional): validates concurrent reservations do not oversell.
// Requires POSTGRES_DSN pointing to a test database.
func TestConcurrentCreatePendingOrder_DoesNotOversell(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	ctx := context.Background()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	// Ensure schema exists.
	// Expect migrations already applied by the environment running this test.

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()

	now := time.Now()

	_, err = sqlDB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$4)
on conflict do nothing
`, customerID, "itest:ssh", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,true,$9,$10)
on conflict (sku) do nothing
`, productID, "ITEST-SKU", "ITest Beer", "ITest", "IPA", 5.0, 330, "INVIMA-ITEST", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',1000,$2)
on conflict do nothing
`, productID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqlDB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,5,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0, safety_stock=0, updated_at=now()
`, productID)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(
		db.New(sqlDB),
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
			if err.Error() == "conflict" {
				conflicts++
				return
			}
		}()
	}
	wg.Wait()

	if success != 5 {
		t.Fatalf("expected 5 successes, got %d (conflicts=%d)", success, conflicts)
	}
}

