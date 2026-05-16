package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	"sip-happens/services/core-api/internal/testutil"
)

func TestGetOrder_ScopedToCustomerFingerprint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const (
		fpA = "SHA256:customer-a"
		fpB = "SHA256:customer-b"
	)
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-a', $1, now(), now()), ('cust-b', $2, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fpA, fpB)
	if err != nil {
		t.Fatalf("seed customers: %v", err)
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, created_at, updated_at)
values ('order-a', 'SH-ORDERA01', 'PENDING_PAYMENT', 'cust-a', 1000, 0, 1000, 'COP', now(), now())
`)
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}

	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)

	srv := &Server{
		DB:              tdb.DB,
		Orders:          ordersSvc,
		Customers:       customers.NewService(tdb.DB, customers.NewSQLRepository()),
		CoreAPIHMACKeys: map[string]string{"dev": "test-secret"},
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/orders/order-a", nil)
	req.Header.Set(auth.HeaderFingerprint, fpB)
	req.Header.Set(auth.HeaderKeyID, "dev")
	if err := SignRequest(req, []byte("test-secret"), "dev", fpB); err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-customer read, got %d", resp.StatusCode)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	if env.Error.Code != "not_found" {
		t.Fatalf("expected not_found, got %q", env.Error.Code)
	}

	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/orders/order-a", nil)
	req2.Header.Set(auth.HeaderFingerprint, fpA)
	req2.Header.Set(auth.HeaderKeyID, "dev")
	if err := SignRequest(req2, []byte("test-secret"), "dev", fpA); err != nil {
		t.Fatal(err)
	}

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for owner, got %d", resp2.StatusCode)
	}
}
