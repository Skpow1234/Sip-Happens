package orders

import (
	"context"
	"errors"
	"sync/atomic"
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

type benchFixture struct {
	svc        Service
	customerID domain.CustomerID
	productID  domain.ProductID
}

func setupBenchFixture(b *testing.B, onHand int) benchFixture {
	b.Helper()
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		b.Skipf("db not available: %v", err)
	}
	b.Cleanup(func() { _ = tdb.CloseFn() })

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	now := time.Now()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$3)
on conflict do nothing
`, customerID, "bench:ssh", now)
	if err != nil {
		b.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,true,$9,$9)
on conflict (sku) do nothing
`, productID, "BENCH-SKU", "Bench Beer", "Bench", "IPA", 5.0, 330, "INVIMA-BENCH", now)
	if err != nil {
		b.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',1000,$2)
on conflict do nothing
`, productID, now)
	if err != nil {
		b.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,$2,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0, safety_stock=0, updated_at=now()
`, productID, onHand)
	if err != nil {
		b.Fatal(err)
	}

	svc := NewService(
		db.New(tdb.DB),
		NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)
	return benchFixture{
		svc:        svc,
		customerID: domain.CustomerID(customerID),
		productID:  domain.ProductID(productID),
	}
}

func (f benchFixture) checkout(ctx context.Context) error {
	_, _, err := f.svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
		CustomerID: f.customerID,
		Currency:   "COP",
		City:       "Bogotá",
		Items: []CreatePendingOrderItem{
			{ProductID: f.productID, Qty: 1},
		},
	})
	return err
}

func BenchmarkCreatePendingOrder_Reserve(b *testing.B) {
	ctx := context.Background()
	fx := setupBenchFixture(b, b.N+1000)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := fx.checkout(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// Parallel checkouts against limited stock (inventory reservation contention).
func BenchmarkCreatePendingOrder_ConcurrentContention(b *testing.B) {
	ctx := context.Background()
	const stock = 200
	fx := setupBenchFixture(b, stock)

	var ok, conflict atomic.Int64
	b.SetParallelism(8)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := fx.checkout(ctx)
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, domain.ErrConflict):
				conflict.Add(1)
			default:
				b.Fatal(err)
			}
		}
	})
	b.ReportMetric(float64(ok.Load()), "orders_ok")
	b.ReportMetric(float64(conflict.Load()), "conflicts")
}

// Parallel checkouts with ample inventory (throughput under concurrency).
func BenchmarkCreatePendingOrder_Concurrent(b *testing.B) {
	ctx := context.Background()
	fx := setupBenchFixture(b, b.N*8+1000)

	b.SetParallelism(8)
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := fx.checkout(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}
