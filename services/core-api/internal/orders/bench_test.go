package orders

import (
	"context"
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

func BenchmarkCreatePendingOrder_Reserve(b *testing.B) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		b.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	now := time.Now()

	// Seed minimal catalog + inventory.
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
	// Large on_hand so the benchmark doesn't fail due to depletion.
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,$2,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0, safety_stock=0, updated_at=now()
`, productID, b.N+1000)
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

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
			CustomerID: domain.CustomerID(customerID),
			Currency:   "COP",
			City:       "Bogotá",
			Items: []CreatePendingOrderItem{
				{ProductID: domain.ProductID(productID), Qty: 1},
			},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

