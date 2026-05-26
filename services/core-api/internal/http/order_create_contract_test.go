package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/audit"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
	"sip-happens/services/core-api/internal/testutil"
)

func TestOrderCreate_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:order-create-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-order-create', $1, now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	productID := "p_order_create"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-OC','Beer Direct','Brewery','Lager',5.0,330,'INVIMA-OC',true,now(),now())
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
values ($1,15,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand
`, productID)

	auditStore := audit.NewSQLStore()
	ordersSvc := orders.NewServiceWithOutbox(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
		nil,
		auditStore,
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

	body, _ := json.Marshal(map[string]any{
		"currency":        "COP",
		"privacy_consent": true,
		"birth_date":      "1992-03-10",
		"city":            "Bogota",
		"neighborhood":    "Chapinero",
		"address_line1":   "Calle 20",
		"phone":           "3001112233",
		"legal_id":        "123456",
		"full_name":       "Direct Buyer",
		"items":           []map[string]any{{"product_id": productID, "qty": 1}},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "order-create-contract-1")
	req.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(req, []byte("test-secret"), "dev", fp); err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /orders: %d %s", resp.StatusCode, string(b))
	}
	var out struct {
		Order struct {
			ID         string `json:"id"`
			DisplayRef string `json:"display_ref"`
			Status     string `json:"status"`
		} `json:"order"`
		Stripe struct {
			URL string `json:"URL"`
		} `json:"stripe"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Order.Status != "PENDING_PAYMENT" || out.Order.DisplayRef == "" || out.Stripe.URL == "" {
		t.Fatalf("unexpected: %+v", out)
	}

	var ageCount int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from age_verifications where order_id=$1 and method='order_create'
`, out.Order.ID).Scan(&ageCount); err != nil {
		t.Fatal(err)
	}
	if ageCount != 1 {
		t.Fatalf("expected order_create age verification, got %d", ageCount)
	}
}
