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

func TestPaymentMethodsCollect_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:pm-collect-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, email, created_at, updated_at)
values ('cust-pm-collect', $1, 'PM User', 'pm@example.com', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"success_url": "https://example.com/ok",
		"cancel_url":  "https://example.com/cancel",
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/payment-methods/collect", bytes.NewReader(body))
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST collect: %d", resp.StatusCode)
	}
	var out struct {
		Collect struct {
			URL               string `json:"url"`
			CheckoutSessionID string `json:"checkout_session_id"`
		} `json:"collect"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Collect.URL == "" || out.Collect.CheckoutSessionID == "" {
		t.Fatalf("collect: %#v", out.Collect)
	}

	var stripeID string
	if err := tdb.DB.QueryRowContext(ctx, `
select coalesce(stripe_customer_id, '') from customers where id='cust-pm-collect'
`).Scan(&stripeID); err != nil {
		t.Fatal(err)
	}
	if stripeID == "" {
		t.Fatal("expected stripe_customer_id persisted")
	}
}
