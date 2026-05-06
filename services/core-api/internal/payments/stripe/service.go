package stripe

import (
	"context"
	"time"

	"sip-happens/pkg/domain"
)

type Service interface {
	// CreateCheckout starts an off-terminal Stripe hosted checkout.
	// It must set Stripe metadata to include internal order_id and use idempotency keys.
	CreateCheckout(ctx context.Context, in CreateCheckoutInput) (*CreateCheckoutOutput, error)
}

type CreateCheckoutInput struct {
	OrderID  domain.OrderID
	Currency string
	Total    int64 // minor units (COP on Stripe == pesos)

	ExpiresAt time.Time

	SuccessURL string
	CancelURL  string
}

type CreateCheckoutOutput struct {
	CheckoutSessionID string
	PaymentIntentID   string
	URL               string
}

