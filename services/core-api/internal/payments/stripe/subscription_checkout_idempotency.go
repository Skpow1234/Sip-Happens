package stripe

import "sip-happens/pkg/domain"

// SubscriptionCheckoutIdempotencyKey is the Stripe idempotency key for subscription Checkout Sessions.
func SubscriptionCheckoutIdempotencyKey(subscriptionID domain.SubscriptionID) string {
	return "subscription_checkout:" + string(subscriptionID)
}
