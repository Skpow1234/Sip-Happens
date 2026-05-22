package stripe

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestPaymentMethodCollectIdempotencyKey(t *testing.T) {
	got := PaymentMethodCollectIdempotencyKey(domain.CustomerID("cust-1"))
	if got != "pm-collect:cust-1" {
		t.Fatalf("got %q", got)
	}
}
