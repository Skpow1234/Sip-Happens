package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/testutil"
	"testing"
	"time"
)

// Contract test: POST /checkout returns order + stripe.url shape expected by ssh-gateway.
func TestCheckoutResponseShape_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping integration test (need POSTGRES_DSN): %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "p_checkout_contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-CHK','Beer Checkout','Brewery','Lager',5.0,330,'INVIMA-CHK',true,now(),now())
on conflict (id) do nothing
`, productID)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',12000,now())
on conflict do nothing
`, productID)
	if err != nil {
		t.Fatalf("seed price: %v", err)
	}
	_, _ = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,20,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0
`, productID)

	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)

	srv := &Server{
		DB:                tdb.DB,
		Catalog:           catalog.NewSQLRepository(),
		Orders:            ordersSvc,
		Customers:         customers.NewService(tdb.DB, customers.NewSQLRepository()),
		Admin:             &admin.SQLService{DB: tdb.DB},
		Stripe:            stripepayments.MockClient{},
		CoreAPIHMACKeys:   map[string]string{"dev": "test-secret"},
		CoreAPIHMACSecret: "",
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{
  "currency":"COP",
  "city":"Bogotá",
  "neighborhood":"Chapinero",
  "address_line1":"Calle 1",
  "phone":"3001234567",
  "legal_id":"123",
  "full_name":"Test User",
  "birth_date":"1995-01-01",
  "privacy_consent":true,
  "items":[{"product_id":"` + productID + `","qty":1}]
}`)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "contract-test-key-1")
	req.Header.Set("X-SSH-Fingerprint", "SHA256:contracttest")
	if err := SignRequest(req, []byte("test-secret"), "dev", "SHA256:contracttest"); err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /checkout: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(b))
	}

	var out struct {
		Order struct {
			ID     string `json:"ID"`
			Status string `json:"Status"`
		} `json:"order"`
		Stripe struct {
			URL string `json:"URL"`
		} `json:"stripe"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Order.ID == "" {
		t.Fatal("expected order.ID")
	}
	if out.Order.Status != "PENDING_PAYMENT" {
		t.Fatalf("expected PENDING_PAYMENT, got %q", out.Order.Status)
	}
	if out.Stripe.URL == "" {
		t.Fatal("expected stripe.URL")
	}

	var ageCount int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from age_verifications where order_id=$1 and method='checkout'
`, out.Order.ID).Scan(&ageCount); err != nil {
		t.Fatal(err)
	}
	if ageCount != 1 {
		t.Fatalf("expected 1 age_verification, got %d", ageCount)
	}

	var auditCount int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from audit_events where aggregate_id=$1 and event_type='age.verified'
`, out.Order.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected 1 age.verified audit, got %d", auditCount)
	}
}
