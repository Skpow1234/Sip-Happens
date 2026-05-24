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

func TestSubscriptions_CRUD_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:subscriptions-crud-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-sub-crud', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	productID := "p_sub_crud"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-SUB','Sub Beer','Brewery','Lager',5.0,330,'INVIMA-SUB',true,now(),now())
on conflict (id) do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',12000,now())
on conflict do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")

	createBody, _ := json.Marshal(map[string]any{
		"product_id": productID,
		"qty":        2,
		"interval":   "month",
		"currency":   "COP",
	})
	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/subscriptions", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "sub-crud-create-1")
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
		t.Fatalf("POST /subscriptions: %d %s", createResp.StatusCode, string(b))
	}
	var created struct {
		Subscription struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Interval string `json:"interval"`
			Qty      int    `json:"qty"`
		} `json:"subscription"`
		Stripe struct {
			URL string `json:"url"`
		} `json:"stripe"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Subscription.ID == "" || created.Subscription.Status != "PENDING" {
		t.Fatalf("created: %#v", created.Subscription)
	}
	if created.Stripe.URL == "" {
		t.Fatal("expected stripe checkout url")
	}

	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/subscriptions", nil)
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

	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/subscriptions/"+created.Subscription.ID, nil)
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

	updateBody, _ := json.Marshal(map[string]any{"qty": 3})
	updateReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/subscriptions/"+created.Subscription.ID, bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(updateReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatal(err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(updateResp.Body)
		t.Fatalf("PUT: %d %s", updateResp.StatusCode, string(b))
	}
	var updated struct {
		Subscription struct {
			Qty int `json:"qty"`
		} `json:"subscription"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Subscription.Qty != 3 {
		t.Fatalf("qty: %d", updated.Subscription.Qty)
	}

	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/subscriptions/"+created.Subscription.ID, nil)
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
	var cancelled struct {
		Subscription struct {
			Status string `json:"status"`
		} `json:"subscription"`
	}
	if err := json.NewDecoder(delResp.Body).Decode(&cancelled); err != nil {
		t.Fatal(err)
	}
	if cancelled.Subscription.Status != "CANCELLED" {
		t.Fatalf("status: %s", cancelled.Subscription.Status)
	}
}
