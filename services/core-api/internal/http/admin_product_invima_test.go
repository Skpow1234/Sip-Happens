package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/testutil"
)

func TestAdminPatchProduct_ActiveWithoutInvima_Unprocessable(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "prod-invima-publish-test"
	now := time.Now()
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'INVIMA-PATCH-SKU','Draft Beer','Brew','Lager',5.0,330,'',false,$2,$2)
on conflict (id) do update set invima_reg='', is_active=false, updated_at=excluded.updated_at
`, productID, now)
	if err != nil {
		t.Fatal(err)
	}

	srv := &Server{
		DB:         tdb.DB,
		Admin:      &admin.SQLService{DB: tdb.DB},
		AdminToken: "test-admin",
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"is_active":true}`)
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/admin/products/"+productID, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-admin")
	req.Header.Set("Idempotency-Key", "invima-publish-test-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}

	var row struct {
		active bool
		invima string
	}
	err = tdb.DB.QueryRowContext(ctx, `select is_active, invima_reg from products where id=$1`, productID).
		Scan(&row.active, &row.invima)
	if err != nil {
		t.Fatal(err)
	}
	if row.active {
		t.Fatal("product should remain inactive")
	}
}
