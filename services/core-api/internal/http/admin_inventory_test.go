package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/admin"
	"sip-happens/services/core-api/internal/testutil"
)

func TestAdminInventory_GetShowsReservedAndReservations(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	productID := "prod-inv-admin"
	sku := "INV-ADMIN-SKU"
	now := time.Now().UTC()
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,'Inv Beer','Brew','IPA',6.0,330,'INVIMA-INV',true,$3,$3)
on conflict (id) do nothing
`, productID, sku, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1, 20, 3, 2, $2)
on conflict (product_id) do update set on_hand=20, reserved=3, safety_stock=2, updated_at=excluded.updated_at
`, productID, now)
	if err != nil {
		t.Fatal(err)
	}

	orderID := "ord-inv-res"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-inv','fp-inv',$1,$1) on conflict do nothing
`, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into orders (id, display_ref, customer_id, status, subtotal_minor_units, shipping_minor_units, total_minor_units, currency, created_at, updated_at, expires_at)
values ($1,'SH-INVTEST','cust-inv','PENDING_PAYMENT',0,0,0,'COP',$2,$2,$2)
on conflict (id) do nothing
`, orderID, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory_reservations (order_id, product_id, qty, status, created_at, updated_at)
values ($1,$2,3,'ACTIVE',$3,$3)
on conflict (order_id, product_id) do update set qty=3, status='ACTIVE', updated_at=excluded.updated_at
`, orderID, productID, now)
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

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/inventory?sku="+sku, nil)
	req.Header.Set("Authorization", "Bearer test-admin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inventory list: expected 200, got %d", resp.StatusCode)
	}
	var invBody struct {
		Items []struct {
			OnHand       int `json:"on_hand"`
			Reserved     int `json:"reserved"`
			SafetyStock  int `json:"safety_stock"`
			AvailableQty int `json:"available_qty"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&invBody); err != nil {
		t.Fatal(err)
	}
	if len(invBody.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(invBody.Items))
	}
	row := invBody.Items[0]
	if row.OnHand != 20 || row.Reserved != 3 || row.SafetyStock != 2 {
		t.Fatalf("unexpected counts: on_hand=%d reserved=%d safety=%d", row.OnHand, row.Reserved, row.SafetyStock)
	}
	wantAvail := domain.Inventory{OnHand: 20, Reserved: 3, SafetyStock: 2}.Available()
	if row.AvailableQty != wantAvail {
		t.Fatalf("available_qty: got %d want %d", row.AvailableQty, wantAvail)
	}

	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/inventory/reservations?sku="+sku, nil)
	req2.Header.Set("Authorization", "Bearer test-admin")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("reservations: expected 200, got %d", resp2.StatusCode)
	}
	var resBody struct {
		Reservations []struct {
			OrderID     string `json:"order_id"`
			Qty         int    `json:"qty"`
			OrderStatus string `json:"order_status"`
		} `json:"reservations"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&resBody); err != nil {
		t.Fatal(err)
	}
	if len(resBody.Reservations) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(resBody.Reservations))
	}
	if resBody.Reservations[0].OrderID != orderID || resBody.Reservations[0].Qty != 3 {
		t.Fatalf("unexpected reservation: %+v", resBody.Reservations[0])
	}
	if resBody.Reservations[0].OrderStatus != string(domain.OrderStatusPendingPayment) {
		t.Fatalf("order_status: %s", resBody.Reservations[0].OrderStatus)
	}
}
