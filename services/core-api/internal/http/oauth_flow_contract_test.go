package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"

	"sip-happens/pkg/auth"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/oauthapps"
	"sip-happens/services/core-api/internal/testutil"
)

func TestOAuth_AuthorizationCodeFlow_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:oauth-flow-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-oauth-flow', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	// Provision an OAuth client directly — the public registration API
	// (oauth-app-crud) ships in the next task; for now we test the flow in
	// isolation by seeding the row the way that future endpoint will.
	clientID, err := oauthapps.GenerateClientID()
	if err != nil {
		t.Fatal(err)
	}
	secret, err := oauthapps.GenerateClientSecret()
	if err != nil {
		t.Fatal(err)
	}
	rowUUID, err := ids.UUIDv4()
	if err != nil {
		t.Fatal(err)
	}
	const redirect = "https://app.example.com/callback"
	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris, scopes, confidential,
  created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,true,now(),now())
`,
		"oc_"+strings.ReplaceAll(rowUUID, "-", ""),
		"cust-oauth-flow",
		"contract test app",
		clientID, secret.Prefix, secret.Hash,
		pq.StringArray{redirect},
		pq.StringArray{"catalog:read", "orders:read"},
	)
	if err != nil {
		t.Fatalf("seed oauth client: %v", err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	signingSecret := []byte("test-secret")

	// 1) /oauth/authorize (signed) → returns a code.
	authzBody, _ := json.Marshal(map[string]any{
		"client_id":    clientID,
		"redirect_uri": redirect,
		"scopes":       []string{"catalog:read", "orders:read"},
		"state":        "xyz123",
	})
	authzReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/authorize", bytes.NewReader(authzBody))
	authzReq.Header.Set("Content-Type", "application/json")
	authzReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(authzReq, signingSecret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	authzResp, err := http.DefaultClient.Do(authzReq)
	if err != nil {
		t.Fatal(err)
	}
	defer authzResp.Body.Close()
	if authzResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(authzResp.Body)
		t.Fatalf("POST /oauth/authorize: %d %s", authzResp.StatusCode, string(b))
	}
	var authzOut struct {
		Code      string   `json:"code"`
		State     string   `json:"state"`
		Scopes    []string `json:"scopes"`
		ExpiresAt string   `json:"expires_at"`
	}
	if err := json.NewDecoder(authzResp.Body).Decode(&authzOut); err != nil {
		t.Fatal(err)
	}
	if _, err := oauthapps.ParseAuthorizationCode(authzOut.Code); err != nil {
		t.Fatalf("malformed code: %q (%v)", authzOut.Code, err)
	}
	if authzOut.State != "xyz123" {
		t.Fatalf("state echo mismatch: %q", authzOut.State)
	}
	if len(authzOut.Scopes) != 2 {
		t.Fatalf("scopes: %v", authzOut.Scopes)
	}

	// 2) /oauth/token (public, form-encoded) → returns access_token.
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", authzOut.Code)
	form.Set("redirect_uri", redirect)
	form.Set("client_id", clientID)
	form.Set("client_secret", secret.Raw)
	tokenReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/token", strings.NewReader(form.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(tokenResp.Body)
		t.Fatalf("POST /oauth/token: %d %s", tokenResp.StatusCode, string(b))
	}
	var tokenOut struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenOut); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tokenOut.AccessToken, "pat_") {
		t.Fatalf("access_token shape: %q", tokenOut.AccessToken)
	}
	if tokenOut.TokenType != "Bearer" {
		t.Fatalf("token_type: %q", tokenOut.TokenType)
	}
	if tokenOut.ExpiresIn <= 0 {
		t.Fatalf("expires_in: %d", tokenOut.ExpiresIn)
	}
	if tokenOut.Scope == "" {
		t.Fatal("scope must echo granted scopes")
	}

	// 3) Code is single-use: second redemption must fail with invalid_grant.
	tokenReq2, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/token", strings.NewReader(form.Encode()))
	tokenReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResp2, err := http.DefaultClient.Do(tokenReq2)
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp2.Body.Close()
	if tokenResp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay should be 400, got %d", tokenResp2.StatusCode)
	}
	var oauthErr struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(tokenResp2.Body).Decode(&oauthErr); err != nil {
		t.Fatal(err)
	}
	if oauthErr.Error != "invalid_grant" {
		t.Fatalf("expected invalid_grant, got %q", oauthErr.Error)
	}

	// 4) Bad client secret must be invalid_client (401).
	form2 := url.Values{}
	form2.Set("grant_type", "authorization_code")
	form2.Set("code", "oac_00000000000000000000000000000000_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	form2.Set("redirect_uri", redirect)
	form2.Set("client_id", clientID)
	form2.Set("client_secret", "ocs_00000000000000000000000000000000_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	badReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/token", strings.NewReader(form2.Encode()))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badResp, err := http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatal(err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad secret should be 401, got %d", badResp.StatusCode)
	}
}

func TestOAuth_Authorize_RejectsUnregisteredRedirect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:oauth-flow-redirect-test"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-oauth-redir', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	clientID, _ := oauthapps.GenerateClientID()
	secret, _ := oauthapps.GenerateClientSecret()
	rowUUID, _ := ids.UUIDv4()
	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris, scopes,
  created_at, updated_at
) values ($1,$2,'redirect test',$3,$4,$5,$6,$7,now(),now())
`,
		"oc_"+strings.ReplaceAll(rowUUID, "-", ""),
		"cust-oauth-redir",
		clientID, secret.Prefix, secret.Hash,
		pq.StringArray{"https://allowed.example.com/cb"},
		pq.StringArray{"catalog:read"},
	)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{
		"client_id":    clientID,
		"redirect_uri": "https://attacker.example.com/steal",
		"scopes":       []string{"catalog:read"},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/authorize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(req, []byte("test-secret"), "dev", fp); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for unregistered redirect, got %d", resp.StatusCode)
	}
}
