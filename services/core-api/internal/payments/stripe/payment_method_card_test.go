package stripe

import (
	"testing"

	"github.com/stripe/stripe-go/v82"
)

func TestPaymentMethodCardFromStripe(t *testing.T) {
	pm := &stripe.PaymentMethod{
		ID:   "pm_123",
		Type: stripe.PaymentMethodTypeCard,
		Card: &stripe.PaymentMethodCard{
			Brand:    stripe.PaymentMethodCardBrandVisa,
			Last4:    "4242",
			ExpMonth: 12,
			ExpYear:  2034,
		},
	}
	card := paymentMethodCardFromStripe(pm, "pm_123")
	if card == nil || !card.IsDefault || card.Last4 != "4242" || card.Brand != "visa" {
		t.Fatalf("card: %+v", card)
	}
}
