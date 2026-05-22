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
	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/testutil"
)

func TestCartItemPut_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "p_cart_item_put"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-CITEM','Item Beer','Brewery','Lager',5.0,330,'INVIMA-CI',true,now(),now())
on conflict (id) do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',10000,now())
on conflict do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1, 5, 0, 0, now())
on conflict (product_id) do update set on_hand=excluded.on_hand
`, productID)

	const fp = "SHA256:cart-item-put-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-cart-item', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	pubID := catalog.PublicID(domain.ProductID(productID))
	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")

	putBody, _ := json.Marshal(map[string]int{"qty": 3})
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/items/"+pubID+"?currency=COP", bytes.NewReader(putBody))
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
		t.Fatalf("PUT item: %d", putResp.StatusCode)
	}
	var out struct {
		Cart struct {
			SubtotalMinorUnits int64 `json:"subtotal_minor_units"`
			ItemCount          int   `json:"item_count"`
		} `json:"cart"`
	}
	if err := json.NewDecoder(putResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Cart.SubtotalMinorUnits != 30000 || out.Cart.ItemCount != 3 {
		t.Fatalf("cart: %+v", out.Cart)
	}

	// qty 0 removes line
	delBody, _ := json.Marshal(map[string]int{"qty": 0})
	delReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/items/"+pubID, bytes.NewReader(delBody))
	delReq.Header.Set("Content-Type", "application/json")
	delReq.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(delReq, secret, "dev", fp)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatal(err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE via qty 0: %d", delResp.StatusCode)
	}
}
