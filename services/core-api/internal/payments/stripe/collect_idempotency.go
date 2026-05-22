package stripe

import "sip-happens/pkg/domain"

func StripeCustomerIdempotencyKey(customerID domain.CustomerID) string {
	return "stripe-customer:" + string(customerID)
}

func PaymentMethodCollectIdempotencyKey(customerID domain.CustomerID) string {
	return "pm-collect:" + string(customerID)
}
