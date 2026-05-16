package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/testutil"
)

func TestAdminPutProduct_IfMatchPrecondition(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "prod-etag-test"
	now := time.Now()
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'ETAG-SKU','Etag Beer','Brew','Lager',5.0,330,'INVIMA-ETAG',true,$2,$2)
on conflict (id) do update set updated_at=excluded.updated_at
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

	updatedAt, err := srv.Admin.ProductUpdatedAt(ctx, domain.ProductID(productID))
	if err != nil {
		t.Fatal(err)
	}
	stale := productWeakETag(updatedAt.Add(-time.Hour))

	body := []byte(`{"sku":"ETAG-SKU","name":"Etag Beer","brewery":"Brew","style":"Lager","abv":5,"volume_ml":330,"invima_reg":"INVIMA-ETAG","is_active":true}`)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/admin/products/"+productID, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-admin")
	req.Header.Set("If-Match", stale)
	req.Header.Set("Idempotency-Key", "etag-test-1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d", resp.StatusCode)
	}
}
