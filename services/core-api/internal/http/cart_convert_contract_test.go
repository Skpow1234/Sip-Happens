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
	"sip-happens/services/core-api/internal/addresses"
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

func TestCartConvert_Contract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	const fp = "SHA256:cart-convert-contract"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, phone, legal_id, created_at, updated_at)
values ('cust-cart-conv', $1, 'Convert User', '3001234567', '123', now(), now())
on conflict (ssh_fingerprint) do nothing
`, fp)
	if err != nil {
		t.Fatal(err)
	}

	productID := "p_cart_convert"
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,'SKU-CC','Beer Convert','Brewery','Lager',5.0,330,'INVIMA-CC',true,now(),now())
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
values ($1,20,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, reserved=0
`, productID)

	addrRepo := addresses.NewSQLRepository()
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
		Addresses:         addrRepo,
		Admin:             &admin.SQLService{DB: tdb.DB},
		Stripe:            stripepayments.MockClient{},
		CoreAPIHMACKeys:   map[string]string{"dev": "test-secret"},
		CoreAPIHMACSecret: "",
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	secret := []byte("test-secret")

	// Seed server cart
	cartBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{{"product_id": productID, "qty": 2}},
	})
	putCart, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart", bytes.NewReader(cartBody))
	putCart.Header.Set("Content-Type", "application/json")
	putCart.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(putCart, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	putResp, err := http.DefaultClient.Do(putCart)
	if err != nil {
		t.Fatal(err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT cart: %d", putResp.StatusCode)
	}

	// Saved address on cart
	createAddr, _ := json.Marshal(map[string]any{
		"address_line1": "Calle 10",
		"neighborhood":  "Chapinero",
		"city":          "Bogota",
		"full_name":     "Convert User",
		"phone":         "3001234567",
		"legal_id":      "123",
	})
	addrReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/addresses", bytes.NewReader(createAddr))
	addrReq.Header.Set("Content-Type", "application/json")
	addrReq.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(addrReq, secret, "dev", fp)
	addrResp, err := http.DefaultClient.Do(addrReq)
	if err != nil {
		t.Fatal(err)
	}
	var addrOut struct {
		Address struct {
			ID string `json:"id"`
		} `json:"address"`
	}
	if err := json.NewDecoder(addrResp.Body).Decode(&addrOut); err != nil {
		t.Fatal(err)
	}
	addrResp.Body.Close()

	putAddrBody, _ := json.Marshal(map[string]string{"address_id": addrOut.Address.ID})
	putAddrReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, ts.URL+"/cart/address", bytes.NewReader(putAddrBody))
	putAddrReq.Header.Set("Content-Type", "application/json")
	putAddrReq.Header.Set(auth.HeaderFingerprint, fp)
	_ = SignRequest(putAddrReq, secret, "dev", fp)
	putAddrResp, err := http.DefaultClient.Do(putAddrReq)
	if err != nil {
		t.Fatal(err)
	}
	putAddrResp.Body.Close()

	convertBody, _ := json.Marshal(map[string]any{
		"privacy_consent": true,
		"birth_date":      "1990-06-15",
	})
	convReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/cart/convert?currency=COP", bytes.NewReader(convertBody))
	convReq.Header.Set("Content-Type", "application/json")
	convReq.Header.Set("Idempotency-Key", "cart-convert-contract-1")
	convReq.Header.Set(auth.HeaderFingerprint, fp)
	if err := SignRequest(convReq, secret, "dev", fp); err != nil {
		t.Fatal(err)
	}
	convResp, err := http.DefaultClient.Do(convReq)
	if err != nil {
		t.Fatal(err)
	}
	defer convResp.Body.Close()
	if convResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(convResp.Body)
		t.Fatalf("POST cart/convert: %d %s", convResp.StatusCode, string(b))
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
	if err := json.NewDecoder(convResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Order.Status != "PENDING_PAYMENT" || out.Stripe.URL == "" {
		t.Fatalf("unexpected convert response: %+v", out)
	}

	var cartItems int
	if err := tdb.DB.QueryRowContext(ctx, `
select coalesce(jsonb_array_length(items_json), 0) from customer_carts where customer_id='cust-cart-conv'
`).Scan(&cartItems); err != nil {
		t.Fatal(err)
	}
	if cartItems != 0 {
		t.Fatalf("expected cleared cart items, got %d", cartItems)
	}

	var addrNull bool
	if err := tdb.DB.QueryRowContext(ctx, `
select address_id is null from customer_carts where customer_id='cust-cart-conv'
`).Scan(&addrNull); err != nil {
		t.Fatal(err)
	}
	if !addrNull {
		t.Fatal("expected cleared cart address_id")
	}

	var ageCount int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from age_verifications where order_id=$1 and method='cart_convert'
`, out.Order.ID).Scan(&ageCount); err != nil {
		t.Fatal(err)
	}
	if ageCount != 1 {
		t.Fatalf("expected cart_convert age verification, got %d", ageCount)
	}
}
