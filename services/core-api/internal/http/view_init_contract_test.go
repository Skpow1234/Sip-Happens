package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/testutil"
)

func TestViewInit_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:view-init-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, created_at, updated_at)
values ('cust-view-init', $1, 'Init Tester', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/view/init?catalog_limit=3&orders_limit=2", nil)
	req.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(req, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /view/init: %d %s", resp.StatusCode, string(b))
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: want no-store, got %q", cc)
	}

	var out struct {
		Profile *struct {
			ID       string `json:"id"`
			FullName string `json:"full_name"`
		} `json:"profile"`
		Catalog *struct {
			Currency string `json:"currency"`
			Limit    int    `json:"limit"`
			Items    []any  `json:"items"`
		} `json:"catalog"`
		Cart   *struct{} `json:"cart"`
		Orders *struct {
			Limit  int   `json:"limit"`
			Orders []any `json:"orders"`
		} `json:"orders"`
		Warnings  []any     `json:"warnings"`
		FetchedAt time.Time `json:"fetched_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}

	if out.Profile == nil {
		t.Fatal("profile must always be present")
	}
	if out.Profile.FullName != "Init Tester" {
		t.Errorf("profile.full_name: %q", out.Profile.FullName)
	}
	if out.FetchedAt.IsZero() {
		t.Error("fetched_at must be set")
	}
	if out.Catalog != nil {
		if out.Catalog.Limit != 3 {
			t.Errorf("catalog.limit: want 3, got %d", out.Catalog.Limit)
		}
		if out.Catalog.Currency != "COP" {
			t.Errorf("catalog.currency: want COP, got %q", out.Catalog.Currency)
		}
	}
	if out.Orders != nil && out.Orders.Limit != 2 {
		t.Errorf("orders.limit: want 2, got %d", out.Orders.Limit)
	}
}

func TestViewInit_ClampsPageSizes_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:view-init-clamp"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-view-init-clamp', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/view/init?catalog_limit=99999&orders_limit=-1", nil)
	req.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(req, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var out struct {
		Catalog *struct {
			Limit int `json:"limit"`
		} `json:"catalog"`
		Orders *struct {
			Limit int `json:"limit"`
		} `json:"orders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Catalog != nil && out.Catalog.Limit > 100 {
		t.Errorf("catalog_limit not clamped: got %d", out.Catalog.Limit)
	}
	if out.Orders != nil && out.Orders.Limit != 5 {
		t.Errorf("orders_limit invalid value should default to 5, got %d", out.Orders.Limit)
	}
}

func TestViewInit_Unsigned_Rejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/view/init", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on unsigned request, got %d", resp.StatusCode)
	}
}
