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

func TestCartPaymentMethodPut_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:cart-pm-put-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, stripe_customer_id, created_at, updated_at)
values ('cust-cart-pm', $1, 'Cart PM', 'cus_test_cartpm', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/payment-methods", nil)
	listReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(listReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	listResp, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatal(err)
	}
	var listed struct {
		PaymentMethods []struct {
			ID    string `json:"id"`
			Last4 string `json:"last4"`
		} `json:"payment_methods"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	listResp.Body.Close()
	if len(listed.PaymentMethods) < 1 {
		t.Fatal("expected mock payment method")
	}
	pmID := listed.PaymentMethods[0].ID

	putBody, _ := json.Marshal(map[string]string{"payment_method_id": pmID})
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/payment-method", bytes.NewReader(putBody))
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
		t.Fatalf("PUT cart/payment-method: %d", putResp.StatusCode)
	}
	var out struct {
		Cart struct {
			PaymentMethodID string `json:"payment_method_id"`
			PaymentMethod   struct {
				ID    string `json:"id"`
				Last4 string `json:"last4"`
			} `json:"payment_method"`
		} `json:"cart"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Cart.PaymentMethodID != pmID || out.Cart.PaymentMethod.ID != pmID {
		t.Fatalf("cart payment method: %+v", out.Cart)
	}

	clearBody, _ := json.Marshal(map[string]string{"payment_method_id": ""})
	clearReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/payment-method", bytes.NewReader(clearBody))
	clearReq.Header.Set("Content-Type", "application/json")
	clearReq.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(clearReq, secret, "dev", fp)
	clearResp, err := http.DefaultClient.Do(clearReq)
	if err != nil {
		t.Fatal(err)
	}
	clearResp.Body.Close()
	if clearResp.StatusCode != http.StatusOK {
		t.Fatalf("clear cart payment method: %d", clearResp.StatusCode)
	}
}
