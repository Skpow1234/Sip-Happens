package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/testutil"
)

func TestOAuthClients_CRUD_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:oauth-clients-crud-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-oauth-crud', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")

	// 1) Create.
	createBody, _ := json.Marshal(map[string]any{
		"name":          "ci dashboard",
		"redirect_uris": []string{"https://app.example.com/callback"},
		"scopes":        []string{"catalog:read", "orders:read"},
	})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/clients", bytes.NewReader(createBody))
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
		t.Fatalf("POST /oauth/clients: %d %s", createResp.StatusCode, string(b))
	}
	var createOut struct {
		OAuthClient struct {
			ID           string   `json:"id"`
			ClientID     string   `json:"client_id"`
			Name         string   `json:"name"`
			RedirectURIs []string `json:"redirect_uris"`
			Scopes       []string `json:"scopes"`
			Confidential bool     `json:"confidential"`
		} `json:"oauth_client"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createOut); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(createOut.OAuthClient.ID, "oc_") {
		t.Fatalf("public id: %q", createOut.OAuthClient.ID)
	}
	if !strings.HasPrefix(createOut.OAuthClient.ClientID, "oci_") {
		t.Fatalf("client_id: %q", createOut.OAuthClient.ClientID)
	}
	if !strings.HasPrefix(createOut.ClientSecret, "ocs_") || strings.Count(createOut.ClientSecret, "_") != 2 {
		t.Fatalf("client_secret shape: %q", createOut.ClientSecret)
	}
	if !createOut.OAuthClient.Confidential {
		t.Fatal("confidential should default to true")
	}

	// 2) List includes the new row, never includes the raw secret.
	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/oauth/clients?limit=10", nil)
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
		t.Fatalf("GET /oauth/clients: %d", listResp.StatusCode)
	}
	listBody, _ := io.ReadAll(listResp.Body)
	if strings.Contains(string(listBody), "client_secret") {
		t.Fatal("listing must NOT expose client_secret")
	}
	if strings.Contains(string(listBody), "client_secret_hash") {
		t.Fatal("listing must NOT expose client_secret_hash")
	}
	if !strings.Contains(string(listBody), createOut.OAuthClient.ID) {
		t.Fatalf("created row missing from list: %s", string(listBody))
	}

	// 3) Get by public id, also never exposes secret material.
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/oauth/clients/"+createOut.OAuthClient.ID, nil)
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
	getBody, _ := io.ReadAll(getResp.Body)
	if strings.Contains(string(getBody), "client_secret") {
		t.Fatal("get must NOT expose client_secret")
	}

	// 4) Bad input — empty redirect_uris.
	badBody, _ := json.Marshal(map[string]any{
		"name":          "bad",
		"redirect_uris": []string{},
		"scopes":        []string{"catalog:read"},
	})
	badReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/clients", bytes.NewReader(badBody))
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
		t.Fatalf("empty redirect_uris should be 422, got %d", badResp.StatusCode)
	}

	// 5) Bad input — non-loopback http://.
	badBody2, _ := json.Marshal(map[string]any{
		"name":          "bad",
		"redirect_uris": []string{"http://attacker.example.com/cb"},
		"scopes":        []string{"catalog:read"},
	})
	badReq2, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/clients", bytes.NewReader(badBody2))
	badReq2.Header.Set("Content-Type", "application/json")
	badReq2.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(badReq2, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	badResp2, err := http.DefaultClient.Do(badReq2)
	if err != nil {
		t.Fatal(err)
	}
	defer badResp2.Body.Close()
	if badResp2.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("http:// non-loopback should be 422, got %d", badResp2.StatusCode)
	}

	// 6) Revoke is idempotent and never exposes secret.
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/oauth/clients/"+createOut.OAuthClient.ID, nil)
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
	var delOut struct {
		OAuthClient struct {
			RevokedAt *time.Time `json:"revoked_at"`
		} `json:"oauth_client"`
	}
	delBody, _ := io.ReadAll(delResp.Body)
	if strings.Contains(string(delBody), "client_secret") {
		t.Fatal("delete must NOT expose client_secret")
	}
	if err := json.Unmarshal(delBody, &delOut); err != nil {
		t.Fatal(err)
	}
	if delOut.OAuthClient.RevokedAt == nil {
		t.Fatal("revoked_at must be set after DELETE")
	}
	firstRevokeAt := *delOut.OAuthClient.RevokedAt

	// Replay — same revoked_at preserved.
	delReq2, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/oauth/clients/"+createOut.OAuthClient.ID, nil)
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
		t.Fatalf("idempotent DELETE: %d", delResp2.StatusCode)
	}
	var delOut2 struct {
		OAuthClient struct {
			RevokedAt *time.Time `json:"revoked_at"`
		} `json:"oauth_client"`
	}
	if err := json.NewDecoder(delResp2.Body).Decode(&delOut2); err != nil {
		t.Fatal(err)
	}
	if delOut2.OAuthClient.RevokedAt == nil || !delOut2.OAuthClient.RevokedAt.Equal(firstRevokeAt) {
		t.Fatalf("revoked_at should be preserved across idempotent DELETEs (first=%v, second=%v)",
			firstRevokeAt, delOut2.OAuthClient.RevokedAt)
	}
}

func TestOAuthClients_OwnershipIsolation_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const ownerFP = "SHA256:oauth-owner"
	const otherFP = "SHA256:oauth-other"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-oauth-owner', $1, now(), now()),
       ('cust-oauth-other', $2, now(), now())
on conflict (ssh_fingerprint) do nothing
`, ownerFP, otherFP)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	// Owner creates a client.
	body, _ := json.Marshal(map[string]any{
		"name":          "owner-only",
		"redirect_uris": []string{"https://owner.example.com/cb"},
		"scopes":        []string{"catalog:read"},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/oauth/clients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.HeaderFingerprint, ownerFP)
	if err := SignRequest(req, secret, "dev", ownerFP); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}
	var out struct {
		OAuthClient struct {
			ID string `json:"id"`
		} `json:"oauth_client"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}

	// Other customer must not be able to GET it.
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/oauth/clients/"+out.OAuthClient.ID, nil)
	getReq.Header.Set(auth.HeaderFingerprint, otherFP)
	if err := SignRequest(getReq, secret, "dev", otherFP); err != nil {
		t.Fatal(err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-customer GET must be 404, got %d", getResp.StatusCode)
	}

	// Other customer must not be able to DELETE it either.
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/oauth/clients/"+out.OAuthClient.ID, nil)
	delReq.Header.Set(auth.HeaderFingerprint, otherFP)
	if err := SignRequest(delReq, secret, "dev", otherFP); err != nil {
		t.Fatal(err)
	}
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-customer DELETE must be 404, got %d", delResp.StatusCode)
	}
}
