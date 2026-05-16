package customers

import (
	"context"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/testutil"
)

func TestRecordAgeVerification_InsertsRow(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	customerID, _ := ids.UUIDv4()
	orderID, _ := ids.UUIDv4()
	now := time.Now()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,$3,$4) on conflict do nothing
`, customerID, "age-verif:test", now, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, city, created_at, updated_at)
values ($1,'SH-AGE','PENDING_PAYMENT',$2,1000,0,1000,'COP','Bogotá',$3,$3)
`, orderID, customerID, now)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewService(tdb.DB, NewSQLRepository())
	if err := svc.RecordAgeVerification(ctx, domain.CustomerID(customerID), domain.OrderID(orderID), "1990-05-01", "checkout"); err != nil {
		t.Fatal(err)
	}

	var method string
	var orderRef string
	if err := tdb.DB.QueryRowContext(ctx, `
select method, order_id from age_verifications where customer_id=$1 order by created_at desc limit 1
`, customerID).Scan(&method, &orderRef); err != nil {
		t.Fatal(err)
	}
	if method != "checkout" || orderRef != orderID {
		t.Fatalf("got method=%q order=%q", method, orderRef)
	}
}

func TestRecordAgeVerification_RejectsMinor(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	svc := NewService(tdb.DB, NewSQLRepository())
	err = svc.RecordAgeVerification(ctx, domain.CustomerID("c"), domain.OrderID("o"), "2015-01-01", "checkout")
	if err != domain.ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
