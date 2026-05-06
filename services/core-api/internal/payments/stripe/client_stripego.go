package stripe

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	paymentintent "github.com/stripe/stripe-go/v82/paymentintent"
)

type StripeGoClient struct {
	SecretKey string
}

func (c StripeGoClient) CreateCheckout(ctx context.Context, in CreateCheckoutInput) (*CreateCheckoutOutput, error) {
	if c.SecretKey == "" {
		return nil, errors.New("missing stripe secret key")
	}
	if in.OrderID == "" {
		return nil, errors.New("missing order id")
	}
	if in.Currency == "" {
		return nil, errors.New("missing currency")
	}
	if in.Total <= 0 {
		return nil, errors.New("invalid total")
	}
	if in.SuccessURL == "" || in.CancelURL == "" {
		return nil, errors.New("missing success/cancel url")
	}

	stripe.Key = c.SecretKey

	// Stripe expects unix seconds for expires_at.
	expUnix := in.ExpiresAt.Unix()

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(in.SuccessURL),
		CancelURL:  stripe.String(in.CancelURL),
		ExpiresAt:  stripe.Int64(expUnix),
		Currency:   stripe.String(in.Currency),
		Metadata: map[string]string{
			"order_id": string(in.OrderID),
		},
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(in.Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Order %s", in.OrderID)),
					},
					UnitAmount: stripe.Int64(in.Total),
				},
			},
		},
	}

	// Use a deterministic idempotency key per order.
	params.SetIdempotencyKey(fmt.Sprintf("checkout:%s", in.OrderID))

	s, err := checkoutsession.New(params)
	if err != nil {
		return nil, err
	}

	out := &CreateCheckoutOutput{
		CheckoutSessionID: s.ID,
		URL:               s.URL,
	}
	if s.PaymentIntent != nil {
		out.PaymentIntentID = s.PaymentIntent.ID
	}
	// Some Stripe objects may not be expanded; ensure safe.
	if out.PaymentIntentID == "" && s.PaymentIntent != nil && s.PaymentIntent.ID != "" {
		out.PaymentIntentID = s.PaymentIntent.ID
	}

	// Defensive: If ExpiresAt is in the past, Stripe may reject; validate earlier.
	_ = time.Second

	return out, nil
}

func (c StripeGoClient) PaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	if c.SecretKey == "" {
		return "", errors.New("missing stripe secret key")
	}
	if paymentIntentID == "" {
		return "", errors.New("missing payment intent id")
	}
	stripe.Key = c.SecretKey

	pi, err := paymentintent.Get(paymentIntentID, nil)
	if err != nil {
		return "", err
	}
	_ = ctx
	return string(pi.Status), nil
}

