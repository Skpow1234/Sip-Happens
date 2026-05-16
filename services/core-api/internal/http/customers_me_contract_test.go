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

// Contract: GET /customers/me returns { "customer": { ID, ... } } for caller fingerprint.
func TestGetCustomersMe_Shape_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN or Docker): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:customers-me-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, city, created_at, updated_at)
values ('cust-me-contract', $1, 'Me Contract', 'Bogotá', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/customers/me", nil)
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
		Customer struct {
			ID       string `json:"ID"`
			FullName string `json:"FullName"`
			City     string `json:"City"`
		} `json:"customer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Customer.ID == "" {
		t.Fatal("expected customer.ID")
	}
	if out.Customer.FullName == "" {
		t.Fatal("expected customer.FullName")
	}
}
