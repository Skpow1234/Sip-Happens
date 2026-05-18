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

// Contract test: /catalog returns snake_case product.id (public) and nested product/price.
func TestCatalogResponseShape_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "550e8400-e29b-41d4-a716-446655440099"
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
				ID   string `json:"id"`
				SKU  string `json:"sku"`
				Name string `json:"name"`
			} `json:"product"`
			Price struct {
				Currency         string `json:"currency"`
				AmountMinorUnits int64  `json:"amount_minor_units"`
			} `json:"price"`
			AvailableQty int `json:"available_qty"`
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
	wantPublicID := "prd_550e8400e29b41d4a716446655440099"
	if out.Items[0].Product.ID != wantPublicID {
		t.Fatalf("expected public id %s, got=%s", wantPublicID, out.Items[0].Product.ID)
	}
	if out.Items[0].Product.SKU != "SKU-1" {
		t.Fatalf("sku: %q", out.Items[0].Product.SKU)
	}
	if out.Items[0].Price.Currency != "COP" || out.Items[0].Price.AmountMinorUnits != 10000 {
		t.Fatalf("unexpected price: %#v", out.Items[0].Price)
	}

	// Detail by public id
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/catalog/"+wantPublicID, nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET detail: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("detail status: %d", resp2.StatusCode)
	}
	var detail struct {
		Product struct {
			ID  string `json:"id"`
			SKU string `json:"sku"`
		} `json:"product"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Product.ID != wantPublicID || detail.Product.SKU != "SKU-1" {
		t.Fatalf("detail product: %#v", detail.Product)
	}

	// Detail by SKU (legacy)
	req3, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/catalog/SKU-1", nil)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("GET detail sku: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("detail by sku status: %d", resp3.StatusCode)
	}
}
