package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	"sip-happens/services/core-api/internal/testutil"
)

func TestAdminOrderDetail_RecordsPIIViewAudit(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	orderID := "ord-pii-audit"
	customerID := "cust-pii-audit"
	now := time.Now()
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, phone, legal_id, created_at, updated_at)
values ($1,'fp-pii','Test User','300','123', $2,$2) on conflict do nothing
`, customerID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, phone, legal_id, full_name, created_at, updated_at)
values ($1,'SH-PII','PAID',$2,1000,0,1000,'COP','300','123','Test User',$3,$3) on conflict do nothing
`, orderID, customerID, now)
	if err != nil {
		t.Fatal(err)
	}

	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)
	srv := &Server{
		DB:          tdb.DB,
		Orders:      ordersSvc,
		Admin:       &admin.SQLService{DB: tdb.DB},
		AdminToken:  "test-admin",
		AdminTokens: []string{"test-admin"},
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/orders/"+orderID, nil)
	req.Header.Set("Authorization", "Bearer test-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}

	var n int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from audit_events where aggregate_id=$1 and event_type='customer.pii_viewed'
`, orderID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 pii audit, got %d", n)
	}
}
