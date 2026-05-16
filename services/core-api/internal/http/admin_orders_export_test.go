package http

import (
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/testutil"
)

func TestAdminOrdersExport_CSV(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	orderID := "order-export-csv-test"
	customerID := "cust-export-csv"
	now := time.Now()
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,'fp-export',$2,$2) on conflict (id) do nothing
`, customerID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, created_at, updated_at)
values ($1,'SH-EXPORT','PAID',$2,5000,1000,6000,'COP',$3,$3)
on conflict (id) do nothing
`, orderID, customerID, now)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	srv.AdminToken = "test-admin"
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/orders/export?limit=10", nil)
	req.Header.Set("Authorization", "Bearer test-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("content-type %q", ct)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected data rows, got %d", len(rows))
	}
	found := false
	for _, row := range rows[1:] {
		if len(row) > 0 && row[0] == orderID {
			found = true
			if row[1] != "SH-EXPORT" {
				t.Fatalf("display_ref: %q", row[1])
			}
			if row[2] != string(domain.OrderStatusPaid) {
				t.Fatalf("status: %q", row[2])
			}
		}
	}
	if !found {
		t.Fatal("exported CSV missing test order")
	}
}
