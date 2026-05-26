package orders

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/audit"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/payments"
	"sip-happens/services/core-api/internal/testutil"
)

func TestCreatePendingOrder_AuditsInventoryReserved(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	sku := fmt.Sprintf("AUDIT-SKU-%s", productID)
	now := time.Now()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,'audit:inv',$2,$2) on conflict do nothing
`, customerID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,'Audit Beer','Brew','IPA',5.0,330,'INVIMA-AUDIT',true,$3,$3)
`, productID, sku, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from) values ($1,'COP',1000,now())
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,5,0,0,now()) on conflict (product_id) do update set on_hand=5, reserved=0
`, productID)
	if err != nil {
		t.Fatal(err)
	}

	auditStore := audit.NewSQLStore()
	svc := NewServiceWithOutbox(
		db.New(tdb.DB),
		NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
		nil,
		auditStore,
	)

	o, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
		CustomerID: domain.CustomerID(customerID),
		Currency:   "COP",
		City:       "Bogotá",
		Items:      []CreatePendingOrderItem{{ProductID: domain.ProductID(productID), Qty: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var n int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from audit_events where aggregate_id=$1 and event_type='inventory.reserved'
`, string(o.ID)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 reserved audit, got %d", n)
	}
}

func TestCancelPendingOrder_AuditsInventoryReleased(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	customerID, _ := ids.UUIDv4()
	productID, _ := ids.UUIDv4()
	sku := fmt.Sprintf("AUDIT-REL-%s", productID)
	now := time.Now()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,'audit:rel',$2,$2) on conflict do nothing
`, customerID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,'Beer','Brew','IPA',5.0,330,'INVIMA-AUDIT',true,$3,$3)
`, productID, sku, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from) values ($1,'COP',1000,now())
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,5,0,0,now()) on conflict (product_id) do update set on_hand=5, reserved=0
`, productID)
	if err != nil {
		t.Fatal(err)
	}

	auditStore := audit.NewSQLStore()
	svc := NewServiceWithOutbox(
		db.New(tdb.DB),
		NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
		nil,
		auditStore,
	)

	o, _, err := svc.CreatePendingOrder(ctx, CreatePendingOrderInput{
		CustomerID: domain.CustomerID(customerID),
		Currency:   "COP",
		City:       "Bogotá",
		Items:      []CreatePendingOrderItem{{ProductID: domain.ProductID(productID), Qty: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelPendingOrder(ctx, o.ID, "test_cancel"); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from audit_events where aggregate_id=$1 and event_type='inventory.released'
`, string(o.ID)).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 released audit, got %d", n)
	}
}
