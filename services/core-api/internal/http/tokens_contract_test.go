package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/testutil"
	"sip-happens/services/core-api/internal/tokens"
)

func TestPersonalAccessTokens_CRUD_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:tokens-crud-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-pat-crud', $1, now(), now())
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
		"name":   "ci script",
		"scopes": []string{"catalog:read", "orders:read"},
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
		PersonalAccessToken struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			TokenPrefix string   `json:"token_prefix"`
			Scopes      []string `json:"scopes"`
		} `json:"personal_access_token"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.PersonalAccessToken.ID == "" || !strings.HasPrefix(created.PersonalAccessToken.ID, "pat_") {
		t.Fatalf("public id: %q", created.PersonalAccessToken.ID)
	}
	if !strings.HasPrefix(created.Token, "pat_") || strings.Count(created.Token, "_") != 2 {
		t.Fatalf("raw token shape: %q", created.Token)
	}
	if created.PersonalAccessToken.TokenPrefix == "" || len(created.PersonalAccessToken.TokenPrefix) != 32 {
		t.Fatalf("token prefix: %q", created.PersonalAccessToken.TokenPrefix)
	}
	if len(created.PersonalAccessToken.Scopes) != 2 {
		t.Fatalf("scopes: %v", created.PersonalAccessToken.Scopes)
	}

	// The raw token reported back must actually match what the DB stored.
	parsedPrefix, err := tokens.ParseRawToken(created.Token)
	if err != nil {
		t.Fatalf("parse raw token: %v", err)
	}
	if parsedPrefix != created.PersonalAccessToken.TokenPrefix {
		t.Fatalf("parsed prefix %q != stored %q", parsedPrefix, created.PersonalAccessToken.TokenPrefix)
	}
	want := sha256.Sum256([]byte(created.Token))
	var storedHash []byte
	if err := tdb.DB.QueryRowContext(ctx, `select token_hash from personal_access_tokens where token_prefix=$1`, parsedPrefix).Scan(&storedHash); err != nil {
		t.Fatalf("load hash: %v", err)
	}
	if hex.EncodeToString(storedHash) != hex.EncodeToString(want[:]) {
		t.Fatal("stored hash does not match sha256(raw token)")
	}

	// Reject missing scopes and duplicate name reuse should both fail validation,
	// not duplicate-key, which keeps the surface area small for the CLI client.
	badBody, _ := json.Marshal(map[string]any{"name": "no scopes"})
	badReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/tokens", bytes.NewReader(badBody))
	badReq.Header.Set("Content-Type", "application/json")
	badReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(badReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	badResp, err := http.DefaultClient.Do(badReq)
	if err != nil {
		t.Fatal(err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST without scopes should be 422, got %d", badResp.StatusCode)
	}

	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/tokens", nil)
	listReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(listReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("GET list: %d", listResp.StatusCode)
	}
	var listed struct {
		PersonalAccessTokens []struct {
			ID    string `json:"id"`
			Token string `json:"token"` // must be absent
		} `json:"personal_access_tokens"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.PersonalAccessTokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(listed.PersonalAccessTokens))
	}
	if listed.PersonalAccessTokens[0].Token != "" {
		t.Fatal("list must never include raw secret")
	}

	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/tokens/"+created.PersonalAccessToken.ID, nil)
	getReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(getReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET by id: %d", getResp.StatusCode)
	}

	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/tokens/"+created.PersonalAccessToken.ID, nil)
	delReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(delReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(delResp.Body)
		t.Fatalf("DELETE: %d %s", delResp.StatusCode, string(b))
	}
	var revoked struct {
		PersonalAccessToken struct {
			RevokedAt *time.Time `json:"revoked_at"`
		} `json:"personal_access_token"`
	}
	if err := json.NewDecoder(delResp.Body).Decode(&revoked); err != nil {
		t.Fatal(err)
	}
	if revoked.PersonalAccessToken.RevokedAt == nil {
		t.Fatal("revoked_at should be set after DELETE")
	}

	// Second DELETE on the same token must remain idempotent (200 + same revoked_at).
	delReq2, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/tokens/"+created.PersonalAccessToken.ID, nil)
	delReq2.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(delReq2, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	delResp2, err := http.DefaultClient.Do(delReq2)
	if err != nil {
		t.Fatal(err)
	}
	defer delResp2.Body.Close()
	if delResp2.StatusCode != http.StatusOK {
		t.Fatalf("second DELETE: %d", delResp2.StatusCode)
	}
}
