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

	// ListPaymentMethods returns card payment methods attached to the Stripe customer.
	ListPaymentMethods(ctx context.Context, in ListPaymentMethodsInput) ([]PaymentMethodCard, error)

	// GetPaymentMethod returns card metadata when the PM belongs to the Stripe customer.
	GetPaymentMethod(ctx context.Context, in GetPaymentMethodInput) (*PaymentMethodCard, error)

	// DetachPaymentMethod removes a saved card from the Stripe customer.
	DetachPaymentMethod(ctx context.Context, in DetachPaymentMethodInput) error

	// PaymentIntentStatus returns Stripe payment intent status (e.g. "succeeded", "processing", "requires_payment_method").
	PaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error)

	// CreateSubscriptionCheckout starts hosted Checkout in subscription mode.
	CreateSubscriptionCheckout(ctx context.Context, in CreateSubscriptionCheckoutInput) (*CreateSubscriptionCheckoutOutput, error)

	// CancelSubscription cancels an active Stripe subscription at period end or immediately.
	CancelSubscription(ctx context.Context, stripeSubscriptionID string) error

	// CreateRefund issues a refund against an existing payment intent.
	// Stripe-side idempotency uses in.IdempotencyKey verbatim; callers are
	// expected to derive a stable key like sha256(order_id|attempt_number)
	// so retries of the same logical refund don't double-pay. AmountMinorUnits
	// may be less than the original charge for partials; zero or unset means
	// "full refund of remaining balance" per Stripe's defaults.
	CreateRefund(ctx context.Context, in CreateRefundInput) (*CreateRefundOutput, error)
}

// CreateRefundInput is the request shape for stripe.CreateRefund. We keep
// it minimal — the underlying Stripe API has dozens of optional fields,
// but for the v1 admin partial-refund flow we only need amount + reason +
// idempotency.
type CreateRefundInput struct {
	PaymentIntentID  string
	AmountMinorUnits int64  // 0 = full refund of remaining balance
	Reason           string // optional, free-form (Stripe accepts requested_by_customer | duplicate | fraudulent | "")
	IdempotencyKey   string // required; usually sha256(order_id|attempt)
	OrderID          domain.OrderID
}

// CreateRefundOutput mirrors the subset of the Stripe refund object we
// store locally. Status is the raw Stripe value ("pending" | "succeeded"
// | "failed" | "canceled"); the service layer normalizes it before
// writing to `refunds.status`.
type CreateRefundOutput struct {
	StripeRefundID string
	Status         string
	FailureReason  string
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

	// StripeCustomerID pre-fills the hosted Checkout page for saved payment methods.
	StripeCustomerID string
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

type ListPaymentMethodsInput struct {
	StripeCustomerID string
}

type GetPaymentMethodInput struct {
	StripeCustomerID string
	PaymentMethodID  string
}

type DetachPaymentMethodInput struct {
	StripeCustomerID string
	PaymentMethodID  string
}

type CreateSubscriptionCheckoutInput struct {
	SubscriptionID       domain.SubscriptionID
	Currency             string
	ProductName          string
	Qty                  int
	UnitAmountMinorUnits int64
	Interval             domain.SubscriptionInterval
	SuccessURL           string
	CancelURL            string
	StripeCustomerID     string
}

type CreateSubscriptionCheckoutOutput struct {
	CheckoutSessionID string
	URL               string
}
