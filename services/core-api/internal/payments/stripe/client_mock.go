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

