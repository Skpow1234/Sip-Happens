package stripe

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestCheckoutSessionIdempotencyKey_DeterministicPerOrder(t *testing.T) {
	orderID := domain.OrderID("ord_abc123")
	got := CheckoutSessionIdempotencyKey(orderID)
	want := "checkout:ord_abc123"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if CheckoutSessionIdempotencyKey(orderID) != got {
		t.Fatal("expected stable key for same order id")
	}
}
