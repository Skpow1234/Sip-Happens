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

func TestCartDelete_Clear_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:cart-clear-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-cart-clear', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	putBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{{"product_id": "p1", "qty": 1}},
	})
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart", bytes.NewReader(putBody))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(putReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()

	addrBody, _ := json.Marshal(map[string]string{"address_id": "addr_test"})
	putAddr, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/address", bytes.NewReader(addrBody))
	putAddr.Header.Set("Content-Type", "application/json")
	putAddr.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(putAddr, secret, "dev", fp)
	putAddrResp, _ := http.DefaultClient.Do(putAddr)
	if putAddrResp != nil {
		putAddrResp.Body.Close()
	}

	delReq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, ts.URL+"/cart", nil)
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
		t.Fatalf("DELETE cart: %d", delResp.StatusCode)
	}
	var out struct {
		Cart struct {
			ItemCount int `json:"item_count"`
		} `json:"cart"`
	}
	if err := json.NewDecoder(delResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Cart.ItemCount != 0 {
		t.Fatalf("expected empty cart, item_count=%d", out.Cart.ItemCount)
	}

	var itemsLen, addrSet, pmSet int
	err = tdb.DB.QueryRowContext(ctx, `
select coalesce(jsonb_array_length(items_json), 0),
       case when address_id is null then 0 else 1 end,
       case when payment_method_id is null then 0 else 1 end
from customer_carts where customer_id='cust-cart-clear'
`).Scan(&itemsLen, &addrSet, &pmSet)
	if err != nil {
		t.Fatal(err)
	}
	if itemsLen != 0 || addrSet != 0 || pmSet != 0 {
		t.Fatalf("cart not cleared: items=%d addr=%d pm=%d", itemsLen, addrSet, pmSet)
	}
}
