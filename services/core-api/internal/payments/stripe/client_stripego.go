package stripe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v82"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	stripecustomer "github.com/stripe/stripe-go/v82/customer"
	paymentintent "github.com/stripe/stripe-go/v82/paymentintent"
	paymentmethod "github.com/stripe/stripe-go/v82/paymentmethod"
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
	if in.TotalMinorUnits <= 0 {
		return nil, errors.New("invalid total")
	}
	if in.SubtotalMinorUnits < 0 || in.ShippingMinorUnits < 0 {
		return nil, errors.New("invalid amounts")
	}
	if in.SubtotalMinorUnits+in.ShippingMinorUnits != in.TotalMinorUnits {
		return nil, errors.New("total mismatch")
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
	}
	if strings.TrimSpace(in.StripeCustomerID) != "" {
		params.Customer = stripe.String(strings.TrimSpace(in.StripeCustomerID))
	}

	if in.ShippingMinorUnits > 0 && in.SubtotalMinorUnits > 0 {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(in.Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Order %s (items)", in.OrderID)),
					},
					UnitAmount: stripe.Int64(in.SubtotalMinorUnits),
				},
			},
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(in.Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Shipping"),
					},
					UnitAmount: stripe.Int64(in.ShippingMinorUnits),
				},
			},
		}
	} else {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(in.Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Order %s", in.OrderID)),
					},
					UnitAmount: stripe.Int64(in.TotalMinorUnits),
				},
			},
		}
	}

	params.SetIdempotencyKey(CheckoutSessionIdempotencyKey(in.OrderID))

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

func (c StripeGoClient) CreateCustomer(ctx context.Context, in CreateCustomerInput) (*CreateCustomerOutput, error) {
	if c.SecretKey == "" {
		return nil, errors.New("missing stripe secret key")
	}
	if in.CustomerID == "" {
		return nil, errors.New("missing customer id")
	}
	stripe.Key = c.SecretKey

	params := &stripe.CustomerParams{
		Metadata: map[string]string{
			"customer_id": string(in.CustomerID),
		},
	}
	if strings.TrimSpace(in.Email) != "" {
		params.Email = stripe.String(strings.TrimSpace(in.Email))
	}
	if strings.TrimSpace(in.Name) != "" {
		params.Name = stripe.String(strings.TrimSpace(in.Name))
	}
	params.SetIdempotencyKey(StripeCustomerIdempotencyKey(in.CustomerID))

	cust, err := stripecustomer.New(params)
	if err != nil {
		return nil, err
	}
	_ = ctx
	return &CreateCustomerOutput{StripeCustomerID: cust.ID}, nil
}

func (c StripeGoClient) CreatePaymentMethodCollect(ctx context.Context, in CreatePaymentMethodCollectInput) (*CreatePaymentMethodCollectOutput, error) {
	if c.SecretKey == "" {
		return nil, errors.New("missing stripe secret key")
	}
	if in.CustomerID == "" {
		return nil, errors.New("missing customer id")
	}
	if in.StripeCustomerID == "" {
		return nil, errors.New("missing stripe customer id")
	}
	if in.SuccessURL == "" || in.CancelURL == "" {
		return nil, errors.New("missing success/cancel url")
	}

	stripe.Key = c.SecretKey

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSetup)),
		Customer:   stripe.String(in.StripeCustomerID),
		SuccessURL: stripe.String(in.SuccessURL),
		CancelURL:  stripe.String(in.CancelURL),
		PaymentMethodTypes: stripe.StringSlice([]string{
			string(stripe.PaymentMethodTypeCard),
		}),
		Metadata: map[string]string{
			"customer_id": string(in.CustomerID),
			"purpose":     "payment_method_collect",
		},
	}
	params.SetIdempotencyKey(PaymentMethodCollectIdempotencyKey(in.CustomerID))

	s, err := checkoutsession.New(params)
	if err != nil {
		return nil, err
	}
	_ = ctx
	return &CreatePaymentMethodCollectOutput{
		CheckoutSessionID: s.ID,
		URL:               s.URL,
	}, nil
}

func (c StripeGoClient) ListPaymentMethods(ctx context.Context, in ListPaymentMethodsInput) ([]PaymentMethodCard, error) {
	if c.SecretKey == "" {
		return nil, errors.New("missing stripe secret key")
	}
	if in.StripeCustomerID == "" {
		return nil, errors.New("missing stripe customer id")
	}
	stripe.Key = c.SecretKey

	cust, err := stripecustomer.Get(in.StripeCustomerID, nil)
	if err != nil {
		return nil, err
	}
	defaultID := defaultPaymentMethodID(cust)

	params := &stripe.CustomerListPaymentMethodsParams{
		Customer: stripe.String(in.StripeCustomerID),
	}
	params.Type = stripe.String(string(stripe.PaymentMethodTypeCard))

	var out []PaymentMethodCard
	iter := stripecustomer.ListPaymentMethods(params)
	for iter.Next() {
		pm := iter.PaymentMethod()
		if card := paymentMethodCardFromStripe(pm, defaultID); card != nil {
			out = append(out, *card)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []PaymentMethodCard{}
	}
	_ = ctx
	return out, nil
}

func (c StripeGoClient) GetPaymentMethod(ctx context.Context, in GetPaymentMethodInput) (*PaymentMethodCard, error) {
	if c.SecretKey == "" {
		return nil, errors.New("missing stripe secret key")
	}
	if in.StripeCustomerID == "" || in.PaymentMethodID == "" {
		return nil, errors.New("missing stripe customer or payment method id")
	}
	stripe.Key = c.SecretKey

	pm, err := paymentmethod.Get(in.PaymentMethodID, nil)
	if err != nil {
		return nil, err
	}
	if pm.Customer == nil || pm.Customer.ID != in.StripeCustomerID {
		return nil, ErrPaymentMethodNotOwned
	}

	cust, err := stripecustomer.Get(in.StripeCustomerID, nil)
	if err != nil {
		return nil, err
	}
	defaultID := defaultPaymentMethodID(cust)
	_ = ctx
	return paymentMethodCardFromStripe(pm, defaultID), nil
}

func (c StripeGoClient) DetachPaymentMethod(ctx context.Context, in DetachPaymentMethodInput) error {
	if c.SecretKey == "" {
		return errors.New("missing stripe secret key")
	}
	if in.StripeCustomerID == "" || in.PaymentMethodID == "" {
		return errors.New("missing stripe customer or payment method id")
	}
	stripe.Key = c.SecretKey

	pm, err := paymentmethod.Get(in.PaymentMethodID, nil)
	if err != nil {
		return err
	}
	if pm.Customer == nil || pm.Customer.ID != in.StripeCustomerID {
		return ErrPaymentMethodNotOwned
	}
	_, err = paymentmethod.Detach(in.PaymentMethodID, nil)
	_ = ctx
	return err
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

