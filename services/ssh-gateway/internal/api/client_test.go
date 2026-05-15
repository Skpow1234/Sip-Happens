package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCatalog_Unsigned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/catalog" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currency": "COP",
			"items": []map[string]any{
				{
					"Product": map[string]any{"ID": "p1", "SKU": "SKU-1", "Name": "Beer"},
					"Price":   map[string]any{"Currency": "COP", "AmountMinorUnits": int64(1000)},
					"available_qty": 5,
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	page, err := c.GetCatalog(context.Background(), CatalogQuery{Limit: 15})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Product.SKU != "SKU-1" || page.Items[0].AvailableQty != 5 {
		t.Fatalf("unexpected items: %#v", page.Items)
	}
}

func TestCheckout_Signed(t *testing.T) {
	var gotIdem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkout" || r.Method != http.MethodPost {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		gotIdem = r.Header.Get("Idempotency-Key")
		if r.Header.Get("X-SSH-Fingerprint") == "" {
			t.Fatal("missing fingerprint")
		}
		if r.Header.Get("X-SipHappens-Signature") == "" {
			t.Fatal("missing signature")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{"ID": "ord_1", "Status": "PENDING_PAYMENT"},
			"stripe": map[string]any{
				"CheckoutSessionID": "cs_test",
				"PaymentIntentID":   "pi_test",
				"URL":               "https://checkout.stripe.com/pay/cs_test",
			},
		})
	}))
	defer srv.Close()

	c := NewSignedClientV2(srv.URL, "dev", "secret", "SHA256:test")
	res, err := c.Checkout(context.Background(), "idem-abc", CheckoutRequest{
		Currency:     "COP",
		City:         "Bogota",
		Neighborhood: "Centro",
		AddressLine1: "Calle 1",
		Phone:        "300",
		LegalID:      "1",
		FullName:     "User",
		Items:        []CheckoutItem{{ProductID: "p1", Qty: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotIdem != "idem-abc" {
		t.Fatalf("idem=%q", gotIdem)
	}
	if res.Order.ID != "ord_1" || res.Stripe == nil || !strings.Contains(res.Stripe.URL, "stripe.com") {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestParseAPIError(t *testing.T) {
	err := parseAPIError(409, []byte(`{"error":{"code":"conflict","message":"no stock"}}`))
	ae, ok := err.(*APIError)
	if !ok || ae.Code != "conflict" {
		t.Fatalf("got %#v", err)
	}
	if ae.FriendlyMessage() == "" {
		t.Fatal("expected friendly message")
	}
}
