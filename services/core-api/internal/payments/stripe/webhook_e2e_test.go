package stripe

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82/webhook"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/orders"
	"sip-happens/services/core-api/internal/payments"
	"sip-happens/services/core-api/internal/testutil"
)

const testWebhookSecret = "whsec_test_secret"

// E2E: signed checkout.session.completed webhook transitions order to PAID.
// Uses POSTGRES_DSN when set (CI), otherwise Postgres via testcontainers (local Docker).
func TestWebhook_CheckoutSessionCompleted_MarkPaid_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("skipping E2E (need POSTGRES_DSN or Docker): %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	productID, customerID := seedWebhookE2EFixture(t, ctx, tdb)

	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)

	o, _, err := ordersSvc.CreatePendingOrder(ctx, orders.CreatePendingOrderInput{
		CustomerID:         domain.CustomerID(customerID),
		Currency:           "COP",
		City:               "Bogotá",
		Neighborhood:       "Chapinero",
		AddressLine1:       "Calle 1",
		Phone:              "300",
		LegalID:            "1",
		FullName:           "E2E",
		ShippingMinorUnits: 0,
		Items: []orders.CreatePendingOrderItem{
			{ProductID: domain.ProductID(productID), Qty: 1},
		},
	})
	if err != nil {
		t.Fatalf("CreatePendingOrder: %v", err)
	}

	sessionID := "cs_test_e2e_session"
	if err := ordersSvc.SetStripeRefs(ctx, o.ID, sessionID, "pi_test_e2e"); err != nil {
		t.Fatalf("SetStripeRefs: %v", err)
	}

	handler := &WebhookHandler{
		Verifier:   StripeGoVerifier{SigningSecret: testWebhookSecret},
		EventStore: &SQLEventStore{DB: tdb.DB},
		Orders:     ordersSvc,
		OrderRepo:  orders.NewSQLRepository(),
		OrderQuery: tdb.DB,
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	payload := mustMarshal(t, map[string]any{
		"id":   "evt_e2e_completed_1",
		"type": "checkout.session.completed",
		"data": map[string]any{
			"object": map[string]any{
				"id": sessionID,
				"metadata": map[string]string{
					"order_id": string(o.ID),
				},
			},
		},
	})
	sig := signTestWebhook(t, payload, testWebhookSecret)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	got, _, err := ordersSvc.GetOrder(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.Status != domain.OrderStatusPaid {
		t.Fatalf("expected PAID, got %s", got.Status)
	}

	var payCount int
	if err := tdb.DB.QueryRowContext(ctx, `select count(*) from payments where order_id=$1`, string(o.ID)).Scan(&payCount); err != nil {
		t.Fatalf("payments count: %v", err)
	}
	if payCount != 1 {
		t.Fatalf("expected 1 payment row, got %d", payCount)
	}

	var onHand, reserved int
	if err := tdb.DB.QueryRowContext(ctx, `select on_hand, reserved from inventory where product_id=$1`, productID).Scan(&onHand, &reserved); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if onHand != 9 || reserved != 0 {
		t.Fatalf("expected on_hand=9 reserved=0 after pay, got on_hand=%d reserved=%d", onHand, reserved)
	}

	// Idempotent replay of same event id.
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL, bytes.NewReader(payload))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Stripe-Signature", sig)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("webhook replay: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("replay expected 200, got %d", resp2.StatusCode)
	}
}

func TestWebhook_CheckoutSessionExpired_CancelsPending_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tdb, err := testutil.OpenMigratedForTest(ctx, t)
	if err != nil {
		t.Skipf("skipping E2E (need POSTGRES_DSN or Docker): %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	productID, customerID := seedWebhookE2EFixture(t, ctx, tdb)

	ordersSvc := orders.NewService(
		db.New(tdb.DB),
		orders.NewSQLRepository(),
		inventory.NewSQLRepository(),
		catalog.NewSQLRepository(),
		payments.NewSQLRepository(),
	)

	o, _, err := ordersSvc.CreatePendingOrder(ctx, orders.CreatePendingOrderInput{
		CustomerID:   domain.CustomerID(customerID),
		Currency:     "COP",
		City:         "Bogotá",
		Neighborhood: "Chapinero",
		AddressLine1: "Calle 1",
		Phone:        "300",
		LegalID:      "1",
		FullName:     "E2E",
		Items: []orders.CreatePendingOrderItem{
			{ProductID: domain.ProductID(productID), Qty: 2},
		},
	})
	if err != nil {
		t.Fatalf("CreatePendingOrder: %v", err)
	}
	sessionID := "cs_test_e2e_expired"
	_ = ordersSvc.SetStripeRefs(ctx, o.ID, sessionID, "pi_test_e2e_exp")

	handler := &WebhookHandler{
		Verifier: StripeGoVerifier{SigningSecret: testWebhookSecret},
		Orders:   ordersSvc,
		OrderRepo: orders.NewSQLRepository(),
		OrderQuery: tdb.DB,
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	payload := mustMarshal(t, map[string]any{
		"id":   "evt_e2e_expired_1",
		"type": "checkout.session.expired",
		"data": map[string]any{
			"object": map[string]any{
				"id": sessionID,
				"metadata": map[string]string{
					"order_id": string(o.ID),
				},
			},
		},
	})
	sig := signTestWebhook(t, payload, testWebhookSecret)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL, bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("webhook POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	got, _, err := ordersSvc.GetOrder(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.Status != domain.OrderStatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", got.Status)
	}

	var reserved int
	if err := tdb.DB.QueryRowContext(ctx, `select reserved from inventory where product_id=$1`, productID).Scan(&reserved); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if reserved != 0 {
		t.Fatalf("expected reserved=0 after cancel, got %d", reserved)
	}
}

func seedWebhookE2EFixture(t *testing.T, ctx context.Context, tdb *testutil.TestDB) (productID, customerID string) {
	t.Helper()
	customerID, _ = ids.UUIDv4()
	suffix, _ := ids.RandomHex(4)
	productID = "p_webhook_e2e_" + suffix
	_, err := tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,'e2e:fp',now(),now())
`, customerID)
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,'E2E Beer','Brew','Lager',5.0,330,'INVIMA-E2E',true,now(),now())
`, productID, "SKU-"+productID)
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	_, err = tdb.DB.ExecContext(ctx, `insert into prices (product_id, currency, amount_minor_units, valid_from) values ($1,'COP',1000,now())`, productID)
	if err != nil {
		t.Fatalf("seed price: %v", err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,10,0,0,now())
`, productID)
	if err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	return productID, customerID
}

func signTestWebhook(t *testing.T, payload []byte, secret string) string {
	t.Helper()
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    secret,
		Timestamp: time.Now(),
	})
	return signed.Header
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
