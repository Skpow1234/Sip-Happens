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

func TestCartGet_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "p_cart_get_contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-CARTGET','Cart Beer','Brewery','Lager',5.0,330,'INVIMA-CG',true,now(),now())
on conflict (id) do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',15000,now())
on conflict do nothing
`, productID)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1, 10, 0, 0, now())
on conflict (product_id) do update set on_hand=excluded.on_hand
`, productID)

	const fp = "SHA256:cart-get-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-cart-get', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	pubID := catalog.PublicID(domain.ProductID(productID))
	putBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"product_id": pubID, "qty": 2},
		},
	})
	srv := testServer(tdb, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	secret := []byte("test-secret")
	putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart?currency=COP", bytes.NewReader(putBody))
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
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT cart: %d", putResp.StatusCode)
	}

	getReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/cart?currency=COP", nil)
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
		t.Fatalf("GET cart: %d", getResp.StatusCode)
	}
	var out struct {
		Cart struct {
			Currency           string `json:"currency"`
			SubtotalMinorUnits int64  `json:"subtotal_minor_units"`
			ItemCount          int    `json:"item_count"`
			Items              []struct {
				Qty                 int `json:"qty"`
				LineTotalMinorUnits int64 `json:"line_total_minor_units"`
				Product             struct {
					ID string `json:"id"`
				} `json:"product"`
				Price struct {
					Currency         string `json:"currency"`
					AmountMinorUnits int64  `json:"amount_minor_units"`
				} `json:"price"`
			} `json:"items"`
		} `json:"cart"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Cart.Currency != "COP" {
		t.Fatalf("currency: %q", out.Cart.Currency)
	}
	if out.Cart.SubtotalMinorUnits != 30000 {
		t.Fatalf("subtotal: %d", out.Cart.SubtotalMinorUnits)
	}
	if out.Cart.ItemCount != 2 {
		t.Fatalf("item_count: %d", out.Cart.ItemCount)
	}
	if len(out.Cart.Items) != 1 || out.Cart.Items[0].Qty != 2 {
		t.Fatalf("items: %+v", out.Cart.Items)
	}
	if out.Cart.Items[0].Price.AmountMinorUnits != 15000 {
		t.Fatalf("unit price: %d", out.Cart.Items[0].Price.AmountMinorUnits)
	}
}
