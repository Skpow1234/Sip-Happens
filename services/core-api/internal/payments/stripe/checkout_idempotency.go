package stripe

import "sip-happens/pkg/domain"

// CheckoutSessionIdempotencyKey is the Stripe idempotency key for creating a Checkout Session per order.
func CheckoutSessionIdempotencyKey(orderID domain.OrderID) string {
	return "checkout:" + string(orderID)
}
