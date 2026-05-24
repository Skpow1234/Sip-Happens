package stripe

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"sip-happens/pkg/ids"
)

var mockPaymentMethodStore struct {
	mu         sync.Mutex
	byCustomer map[string][]PaymentMethodCard
}

func mockPaymentMethodsForCustomer(stripeCustomerID string) []PaymentMethodCard {
	stripeCustomerID = strings.TrimSpace(stripeCustomerID)
	if stripeCustomerID == "" {
		return nil
	}
	mockPaymentMethodStore.mu.Lock()
	defer mockPaymentMethodStore.mu.Unlock()
	if mockPaymentMethodStore.byCustomer == nil {
		mockPaymentMethodStore.byCustomer = make(map[string][]PaymentMethodCard)
	}
	if cards, ok := mockPaymentMethodStore.byCustomer[stripeCustomerID]; ok {
		out := make([]PaymentMethodCard, len(cards))
		copy(out, cards)
		return out
	}
	seed := []PaymentMethodCard{{
		ID:        "pm_test_visa4242",
		Type:      "card",
		Brand:     "visa",
		Last4:     "4242",
		ExpMonth:  12,
		ExpYear:   2034,
		IsDefault: true,
	}}
	mockPaymentMethodStore.byCustomer[stripeCustomerID] = append([]PaymentMethodCard(nil), seed...)
	out := make([]PaymentMethodCard, len(seed))
	copy(out, seed)
	return out
}

func mockRemovePaymentMethod(stripeCustomerID, paymentMethodID string) bool {
	mockPaymentMethodStore.mu.Lock()
	defer mockPaymentMethodStore.mu.Unlock()
	cards := mockPaymentMethodStore.byCustomer[stripeCustomerID]
	for i, c := range cards {
		if c.ID == paymentMethodID {
			mockPaymentMethodStore.byCustomer[stripeCustomerID] = append(cards[:i], cards[i+1:]...)
			return true
		}
	}
	return false
}

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

func (c MockClient) CreateCustomer(ctx context.Context, in CreateCustomerInput) (*CreateCustomerOutput, error) {
	_ = ctx
	if in.CustomerID == "" {
		return nil, fmt.Errorf("missing customer id")
	}
	suffix, err := ids.RandomHex(6)
	if err != nil {
		return nil, err
	}
	return &CreateCustomerOutput{
		StripeCustomerID: fmt.Sprintf("cus_test_%s", suffix),
	}, nil
}

func (c MockClient) CreatePaymentMethodCollect(ctx context.Context, in CreatePaymentMethodCollectInput) (*CreatePaymentMethodCollectOutput, error) {
	_ = ctx
	if in.CustomerID == "" {
		return nil, fmt.Errorf("missing customer id")
	}
	if in.StripeCustomerID == "" {
		return nil, fmt.Errorf("missing stripe customer id")
	}
	suffix, err := ids.RandomHex(8)
	if err != nil {
		return nil, err
	}
	cs := fmt.Sprintf("cs_setup_%s", suffix)
	return &CreatePaymentMethodCollectOutput{
		CheckoutSessionID: cs,
		URL:               fmt.Sprintf("https://checkout.stripe.com/setup/%s", cs),
	}, nil
}

func (c MockClient) ListPaymentMethods(ctx context.Context, in ListPaymentMethodsInput) ([]PaymentMethodCard, error) {
	_ = ctx
	if in.StripeCustomerID == "" {
		return nil, fmt.Errorf("missing stripe customer id")
	}
	cards := mockPaymentMethodsForCustomer(in.StripeCustomerID)
	if cards == nil {
		return []PaymentMethodCard{}, nil
	}
	return cards, nil
}

func (c MockClient) GetPaymentMethod(ctx context.Context, in GetPaymentMethodInput) (*PaymentMethodCard, error) {
	_ = ctx
	if in.StripeCustomerID == "" || in.PaymentMethodID == "" {
		return nil, fmt.Errorf("missing stripe customer or payment method id")
	}
	for _, card := range mockPaymentMethodsForCustomer(in.StripeCustomerID) {
		if card.ID == in.PaymentMethodID {
			cp := card
			return &cp, nil
		}
	}
	return nil, ErrPaymentMethodNotFound
}

func (c MockClient) DetachPaymentMethod(ctx context.Context, in DetachPaymentMethodInput) error {
	_ = ctx
	if in.StripeCustomerID == "" || in.PaymentMethodID == "" {
		return fmt.Errorf("missing stripe customer or payment method id")
	}
	if !mockRemovePaymentMethod(in.StripeCustomerID, in.PaymentMethodID) {
		return ErrPaymentMethodNotFound
	}
	return nil
}

func (c MockClient) PaymentIntentStatus(ctx context.Context, paymentIntentID string) (string, error) {
	_ = ctx
	_ = paymentIntentID
	// For prototype flows, treat all mock payment intents as succeeded.
	return "succeeded", nil
}

func (c MockClient) CreateSubscriptionCheckout(ctx context.Context, in CreateSubscriptionCheckoutInput) (*CreateSubscriptionCheckoutOutput, error) {
	_ = ctx
	if in.SubscriptionID == "" {
		return nil, fmt.Errorf("missing subscription id")
	}
	if in.Currency == "" {
		return nil, fmt.Errorf("missing currency")
	}
	if in.Qty <= 0 || in.UnitAmountMinorUnits <= 0 {
		return nil, fmt.Errorf("invalid subscription amounts")
	}
	suffix, err := ids.RandomHex(8)
	if err != nil {
		return nil, err
	}
	cs := fmt.Sprintf("cs_sub_%s", suffix)
	return &CreateSubscriptionCheckoutOutput{
		CheckoutSessionID: cs,
		URL:               fmt.Sprintf("https://checkout.stripe.com/subscribe/%s", cs),
	}, nil
}

func (c MockClient) CancelSubscription(ctx context.Context, stripeSubscriptionID string) error {
	_ = ctx
	_ = stripeSubscriptionID
	return nil
}

