package http

import (
	"bytes"
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

func TestBearerPAT_AuthAndScopes_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:bearer-pat-auth-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-bearer-pat', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")

	// Mint a PAT with orders:read only (HMAC still used for minting).
	createBody, _ := json.Marshal(map[string]any{
		"name":   "bearer script",
		"scopes": []string{"orders:read"},
	})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/tokens", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(createReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(createResp.Body)
		t.Fatalf("POST /tokens: %d %s", createResp.StatusCode, string(b))
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Token == "" {
		t.Fatal("empty raw token")
	}

	// GET /orders with Bearer PAT (orders:read) — allowed, no HMAC.
	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/orders", nil)
	listReq.Header.Set("Authorization", "Bearer "+created.Token)
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(listResp.Body)
		t.Fatalf("GET /orders with PAT: %d %s", listResp.StatusCode, string(b))
	}

	// POST /checkout with same PAT — insufficient_scope (needs orders:write).
	checkoutBody, _ := json.Marshal(map[string]any{
		"currency": "COP",
		"items":    []any{},
	})
	checkoutReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/checkout", bytes.NewReader(checkoutBody))
	checkoutReq.Header.Set("Content-Type", "application/json")
	checkoutReq.Header.Set("Authorization", "Bearer "+created.Token)
	checkoutReq.Header.Set("Idempotency-Key", "bearer-pat-scope-deny-1")
	checkoutResp, err := http.DefaultClient.Do(checkoutReq)
	if err != nil {
		t.Fatal(err)
	}
	defer checkoutResp.Body.Close()
	if checkoutResp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(checkoutResp.Body)
		t.Fatalf("POST /checkout with read-only PAT: want 403, got %d %s", checkoutResp.StatusCode, string(b))
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(checkoutResp.Body).Decode(&errBody)
	if errBody.Error.Code != "forbidden" {
		t.Fatalf("error code: %q", errBody.Error.Code)
	}

	// Bad token → 401.
	badReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/orders", nil)
	badReq.Header.Set("Authorization", "Bearer pat_deadbeefdeadbeefdeadbeefdeadbeef_notarealsecretvalue0123456789ab")
	badResp, err := http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatal(err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(badResp.Body)
		t.Fatalf("bad PAT: want 401, got %d %s", badResp.StatusCode, string(b))
	}

	// Fingerprint mismatch → 401.
	mismatchReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/orders", nil)
	mismatchReq.Header.Set("Authorization", "Bearer "+created.Token)
	mismatchReq.Header.Set(auth.HeaderFingerprint, "SHA256:someone-else")
	mismatchResp, err := http.DefaultClient.Do(mismatchReq)
	if err != nil {
		t.Fatal(err)
	}
	defer mismatchResp.Body.Close()
	if mismatchResp.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(mismatchResp.Body)
		t.Fatalf("fingerprint mismatch: want 401, got %d %s", mismatchResp.StatusCode, string(b))
	}

	// Matching fingerprint tag still works.
	okFPReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/orders", nil)
	okFPReq.Header.Set("Authorization", "Bearer "+created.Token)
	okFPReq.Header.Set(auth.HeaderFingerprint, fp)
	okFPResp, err := http.DefaultClient.Do(okFPReq)
	if err != nil {
		t.Fatal(err)
	}
	defer okFPResp.Body.Close()
	if okFPResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(okFPResp.Body)
		t.Fatalf("matching fingerprint: %d %s", okFPResp.StatusCode, string(b))
	}
}

func TestBearerPAT_WriteScope_Cart_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:bearer-pat-cart-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-bearer-cart', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")
	createBody, _ := json.Marshal(map[string]any{
		"name":   "cart script",
		"scopes": []string{"cart:write"},
	})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/tokens", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(createReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(createResp.Body)
		t.Fatalf("POST /tokens: %d %s", createResp.StatusCode, string(b))
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/cart", nil)
	getReq.Header.Set("Authorization", "Bearer "+created.Token)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("GET /cart with cart:write PAT: %d %s", getResp.StatusCode, string(b))
	}

	// orders:read still denied on cart.
	ordersOnlyBody, _ := json.Marshal(map[string]any{
		"name":   "orders only",
		"scopes": []string{"orders:read"},
	})
	ordersOnlyReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/tokens", bytes.NewReader(ordersOnlyBody))
	ordersOnlyReq.Header.Set("Content-Type", "application/json")
	ordersOnlyReq.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(ordersOnlyReq, secret, "dev", fp)
	ordersOnlyResp, err := http.DefaultClient.Do(ordersOnlyReq)
	if err != nil {
		t.Fatal(err)
	}
	defer ordersOnlyResp.Body.Close()
	var ordersOnly struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(ordersOnlyResp.Body).Decode(&ordersOnly)

	denyReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/cart", nil)
	denyReq.Header.Set("Authorization", "Bearer "+ordersOnly.Token)
	denyResp, err := http.DefaultClient.Do(denyReq)
	if err != nil {
		t.Fatal(err)
	}
	defer denyResp.Body.Close()
	if denyResp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(denyResp.Body)
		t.Fatalf("GET /cart with orders:read: want 403, got %d %s", denyResp.StatusCode, string(b))
	}
}
