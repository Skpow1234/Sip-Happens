package stripe

import "sip-happens/pkg/domain"

// NormalizePaymentIntentStatus maps Stripe payment_intent.status to domain payment statuses.
func NormalizePaymentIntentStatus(stripeStatus string) domain.PaymentStatus {
	switch stripeStatus {
	case "succeeded":
		return domain.PaymentStatusSucceeded
	case "canceled":
		return domain.PaymentStatusFailed
	case "processing", "requires_action", "requires_payment_method", "requires_confirmation", "requires_capture":
		return domain.PaymentStatusPending
	default:
		return domain.PaymentStatusPending
	}
}
