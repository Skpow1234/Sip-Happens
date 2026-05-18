package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/testutil"
)

// Contract: GET /customers/me returns snake_case profile under "customer".
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
			ID       string `json:"id"`
			FullName string `json:"full_name"`
			City     string `json:"city"`
		} `json:"customer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Customer.ID == "" {
		t.Fatal("expected customer.id")
	}
	if out.Customer.FullName == "" {
		t.Fatal("expected customer.full_name")
	}
}

func TestProfileAlias_GET_PUT_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN or Docker): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:profile-alias-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, city, created_at, updated_at)
values ('cust-profile-alias', $1, 'Before', 'Medellín', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/profile", nil)
	getReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(getReq, []byte("test-secret"), "dev", fp); err != nil {
		t.Fatal(err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET profile: %d", getResp.StatusCode)
	}
	var getOut struct {
		Profile struct {
			ID       string `json:"id"`
			FullName string `json:"full_name"`
		} `json:"profile"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if getOut.Profile.ID == "" || getOut.Profile.FullName != "Before" {
		t.Fatalf("profile: %#v", getOut.Profile)
	}

	body, _ := json.Marshal(map[string]string{"full_name": "After"})
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/profile", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(putReq, []byte("test-secret"), "dev", fp); err != nil {
		t.Fatal(err)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT profile: %d", putResp.StatusCode)
	}
	var putOut struct {
		Profile struct {
			FullName string `json:"full_name"`
		} `json:"profile"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&putOut); err != nil {
		t.Fatalf("decode put: %v", err)
	}
	if putOut.Profile.FullName != "After" {
		t.Fatalf("full_name: %q", putOut.Profile.FullName)
	}
}
