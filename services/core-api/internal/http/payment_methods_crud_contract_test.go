package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/testutil"
)

func TestPaymentMethods_CRUD_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:pm-crud-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, stripe_customer_id, created_at, updated_at)
values ('cust-pm-crud', $1, 'PM CRUD', 'cus_test_pmcrud', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")

	// GET list
	listReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/payment-methods", nil)
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
		PaymentMethods []struct {
			ID    string `json:"id"`
			Last4 string `json:"last4"`
		} `json:"payment_methods"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listOut); err != nil {
		t.Fatal(err)
	}
	if len(listOut.PaymentMethods) < 1 {
		t.Fatal("expected at least one mock payment method")
	}
	pmID := listOut.PaymentMethods[0].ID

	// GET one
	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/payment-methods/"+pmID, nil)
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
	var getOut struct {
		PaymentMethod struct {
			ID string `json:"id"`
		} `json:"payment_method"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatal(err)
	}
	if getOut.PaymentMethod.ID != pmID {
		t.Fatalf("id mismatch: %s vs %s", getOut.PaymentMethod.ID, pmID)
	}

	// DELETE
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/payment-methods/"+pmID, nil)
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

	// GET one after delete -> 404
	getReq2, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/payment-methods/"+pmID, nil)
	getReq2.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(getReq2, secret, "dev", fp)
	getResp2, err := http.DefaultClient.Do(getReq2)
	if err != nil {
		t.Fatal(err)
	}
	getResp2.Body.Close()
	if getResp2.StatusCode != http.StatusNotFound {
		t.Fatalf("GET after delete: %d", getResp2.StatusCode)
	}
}
