package stripe

import (
	"context"
	"fmt"
	"time"

	"sip-happens/pkg/ids"
)

// MockClient is a development-only Stripe client that generates deterministic-looking
// Checkout Session ids and URLs without contacting Stripe.
//
// Use only for local prototype flows when STRIPE_SECRET_KEY is not available.
type MockClient struct{}

func (c MockClient) CreateCheckout(ctx context.Context, in CreateCheckoutInput) (*CreateCheckoutOutput, error) {
	_ = ctx

	if in.OrderID == "" {
		return nil, fmt.Errorf("missing order id")
	}
	if in.Currency == "" {
		return nil, fmt.Errorf("missing currency")
	}
	if in.TotalMinorUnits <= 0 {
		return nil, fmt.Errorf("invalid total")
	}
	if in.SubtotalMinorUnits < 0 || in.ShippingMinorUnits < 0 {
		return nil, fmt.Errorf("invalid amounts")
	}
	if in.SubtotalMinorUnits+in.ShippingMinorUnits != in.TotalMinorUnits {
		return nil, fmt.Errorf("total mismatch")
	}

	suffix, err := ids.RandomHex(8)
	if err != nil {
		return nil, err
	}

	cs := fmt.Sprintf("cs_test_%s", suffix)
	pi := fmt.Sprintf("pi_test_%s", suffix)

	// ExpiresAt is carried for parity, even though this is a mock.
	_ = in.ExpiresAt
	_ = time.Second

	return &CreateCheckoutOutput{
		CheckoutSessionID: cs,
		PaymentIntentID:   pi,
		URL:               fmt.Sprintf("https://checkout.stripe.com/pay/%s", cs),
	}, nil
}

func (c MockClient) PaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	_ = ctx
	_ = paymentIntentID
	// For prototype flows, treat all mock payment intents as succeeded.
	return "succeeded", nil
}

