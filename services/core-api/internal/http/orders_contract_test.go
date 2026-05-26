package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/testutil"
)

// Contract: GET /orders returns { "orders": [...] } for the caller's fingerprint.
func TestGetOrders_ListShape_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN or Docker): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:orders-list-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, created_at, updated_at)
values ('cust-orders-contract', $1, 'Orders Contract', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, created_at, updated_at)
values ('order-list-1', 'SH-ORDLIST1', 'PAID', 'cust-orders-contract', 5000, 8000, 13000, 'COP', now(), now())
`)
	if err != nil {
		t.Fatalf("seed order: %v", err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/orders", nil)
	req.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(req, []byte("test-secret"), "dev", fp); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out struct {
		Orders []struct {
			ID         string `json:"id"`
			DisplayRef string `json:"display_ref"`
			Status     string `json:"status"`
		} `json:"orders"`
		Limit   int  `json:"limit"`
		Offset  int  `json:"offset"`
		HasMore bool `json:"has_more"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Orders) != 1 || out.Orders[0].ID != "order-list-1" || out.Orders[0].DisplayRef != "SH-ORDLIST1" {
		t.Fatalf("unexpected orders: %+v", out.Orders)
	}
	if out.HasMore {
		t.Fatal("expected has_more false")
	}
}

func TestGetOrder_ByDisplayRef_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:orders-displayref-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-orders-dr', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, status, customer_id, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, created_at, updated_at)
values ('order-dr-uuid', 'SH-DISPLAY01', 'PAID', 'cust-orders-dr', 5000, 8000, 13000, 'COP', now(), now())
`)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/orders/SH-DISPLAY01", nil)
	req.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(req, []byte("test-secret"), "dev", fp); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET by display_ref: %d", resp.StatusCode)
	}
	var out struct {
		Order struct {
			ID         string `json:"id"`
			DisplayRef string `json:"display_ref"`
		} `json:"order"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Order.ID != "order-dr-uuid" || out.Order.DisplayRef != "SH-DISPLAY01" {
		t.Fatalf("order: %+v", out.Order)
	}
}
