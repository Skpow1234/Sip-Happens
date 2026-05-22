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

	// CreateCustomer provisions a Stripe Customer for saved payment methods.
	CreateCustomer(ctx context.Context, in CreateCustomerInput) (*CreateCustomerOutput, error)

	// CreatePaymentMethodCollect starts a hosted Checkout Session in setup mode (save card off-terminal).
	CreatePaymentMethodCollect(ctx context.Context, in CreatePaymentMethodCollectInput) (*CreatePaymentMethodCollectOutput, error)

	// PaymentIntentStatus returns Stripe payment intent status (e.g. "succeeded", "processing", "requires_payment_method").
	PaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error)
}

type CreateCheckoutInput struct {
	OrderID  domain.OrderID
	Currency string
	// SubtotalMinorUnits is merchandise before shipping.
	SubtotalMinorUnits int64
	ShippingMinorUnits int64
	// TotalMinorUnits must equal SubtotalMinorUnits + ShippingMinorUnits.
	TotalMinorUnits int64

	ExpiresAt time.Time

	SuccessURL string
	CancelURL  string
}

type CreateCheckoutOutput struct {
	CheckoutSessionID string
	PaymentIntentID   string
	URL               string
}

type CreateCustomerInput struct {
	CustomerID domain.CustomerID
	Email      string
	Name       string
}

type CreateCustomerOutput struct {
	StripeCustomerID string
}

type CreatePaymentMethodCollectInput struct {
	CustomerID       domain.CustomerID
	StripeCustomerID string
	SuccessURL       string
	CancelURL        string
}

type CreatePaymentMethodCollectOutput struct {
	CheckoutSessionID string
	URL               string
}

