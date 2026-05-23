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

func TestCartAddressPut_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:cart-address-put-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-cart-addr', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	// Create saved address
	createBody, _ := json.Marshal(map[string]any{
		"address_line1": "Calle 1",
		"neighborhood":  "Centro",
		"city":          "Bogota",
	})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/addresses", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(createReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Address struct {
			ID string `json:"id"`
		} `json:"address"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	createResp.Body.Close()
	if created.Address.ID == "" {
		t.Fatal("expected address id")
	}

	// PUT cart address
	putBody, _ := json.Marshal(map[string]string{"address_id": created.Address.ID})
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/address", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(putReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT cart/address: %d", putResp.StatusCode)
	}
	var out struct {
		Cart struct {
			AddressID string `json:"address_id"`
			Address   struct {
				City string `json:"city"`
			} `json:"address"`
		} `json:"cart"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Cart.AddressID != created.Address.ID || out.Cart.Address.City != "Bogotá" {
		t.Fatalf("cart address: %+v", out.Cart)
	}

	// Clear address
	clearBody, _ := json.Marshal(map[string]string{"address_id": ""})
	clearReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/address", bytes.NewReader(clearBody))
	clearReq.Header.Set("Content-Type", "application/json")
	clearReq.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(clearReq, secret, "dev", fp)
	clearResp, err := http.DefaultClient.Do(clearReq)
	if err != nil {
		t.Fatal(err)
	}
	clearResp.Body.Close()
	if clearResp.StatusCode != http.StatusOK {
		t.Fatalf("clear cart address: %d", clearResp.StatusCode)
	}
}
