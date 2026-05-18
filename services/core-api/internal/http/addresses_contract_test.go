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

func TestAddresses_CRUD_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:addresses-crud-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, phone, legal_id, created_at, updated_at)
values ('cust-addr-crud', $1, 'Addr User', '300', 'CC-9', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")

	// POST create
	body, _ := json.Marshal(map[string]any{
		"address_line1": "Calle 50 # 10",
		"neighborhood":  "Laureles",
		"city":          "Medellin",
		"is_default":    true,
	})
	postReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/addresses", bytes.NewReader(body))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(postReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	postResp, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatal(err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("POST: %d", postResp.StatusCode)
	}
	var created struct {
		Address struct {
			ID   string `json:"id"`
			City string `json:"city"`
		} `json:"address"`
	}
	if err := json.NewDecoder(postResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Address.ID == "" || created.Address.City != "Medellín" {
		t.Fatalf("created: %#v", created.Address)
	}

	// GET list
	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/addresses", nil)
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
	var listOut struct {
		Addresses []struct {
			ID string `json:"id"`
		} `json:"addresses"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listOut); err != nil {
		t.Fatal(err)
	}
	if len(listOut.Addresses) < 1 {
		t.Fatal("expected addresses")
	}

	// GET by id
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/addresses/"+created.Address.ID, nil)
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
		t.Fatalf("GET one: %d", getResp.StatusCode)
	}

	// DELETE
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/addresses/"+created.Address.ID, nil)
	delReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(delReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE: %d", delResp.StatusCode)
	}
}
