package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/testutil"
)

// This is a lightweight contract test asserting that `/catalog` returns the JSON
// shape expected by the ssh-gateway client (Product + Price nested objects).
func TestCatalogResponseShape_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "p_test_1"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-1','Beer 1','Brewery','Lager',5.0,330,'INVIMA-1',true,now(),now())
`, productID)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',10000,now())
`, productID)
	if err != nil {
		t.Fatalf("seed price: %v", err)
	}
	_, _ = tdb.DB.ExecContext(ctx, `insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at) values ($1,10,0,0,now())`, productID)

	srv := &Server{
		DB:      tdb.DB,
		Catalog: catalog.NewSQLRepository(),
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/catalog", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /catalog: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out struct {
		Currency string `json:"currency"`
		Items    []struct {
			Product struct {
				ID        string
				SKU       string
				Name      string
				Brewery   string
				Style     string
				ABV       float64
				VolumeML  int
				InvimaReg string
			} `json:"Product"`
			Price struct {
				Currency         string
				AmountMinorUnits int64
			} `json:"Price"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Currency != "COP" {
		t.Fatalf("expected currency COP, got %q", out.Currency)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(out.Items))
	}
	if out.Items[0].Product.ID != productID {
		t.Fatalf("expected product_id=%s, got=%s", productID, out.Items[0].Product.ID)
	}
	if out.Items[0].Price.Currency != "COP" || out.Items[0].Price.AmountMinorUnits != 10000 {
		t.Fatalf("unexpected price: %#v", out.Items[0].Price)
	}
}
