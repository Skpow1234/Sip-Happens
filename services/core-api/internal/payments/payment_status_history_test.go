package payments

import (
	"context"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/testutil"
)

func TestPaymentStatusHistory_RecordAndList(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	orderID := "order-pay-hist-test"
	customerID := "cust-pay-hist-test"
	now := time.Now()
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,'fp-pay-hist',$2,$2)
on conflict (id) do nothing
`, customerID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, city, created_at, updated_at)
values ($1,'SH-TEST-1','PENDING_PAYMENT',$2,1000,0,1000,'COP','Bogotá',$3,$3)
on conflict (id) do nothing
`, orderID, customerID, now)
	if err != nil {
		t.Fatal(err)
	}

	repo := NewSQLRepository()
	if err := repo.RecordStatus(ctx, tdb.DB, domain.PaymentStatusHistoryEntry{
		OrderID:     domain.OrderID(orderID),
		Provider:    domain.PaymentProviderStripe,
		ProviderRef: "cs_test_1",
		Status:      domain.PaymentStatusCreated,
		Source:      "checkout",
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := repo.ListStatusHistoryForOrder(ctx, tdb.DB, domain.OrderID(orderID))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != domain.PaymentStatusCreated {
		t.Fatalf("unexpected history: %+v", rows)
	}
}
