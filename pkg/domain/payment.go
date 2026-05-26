package domain

import "time"

type PaymentID string

type PaymentProvider string

const (
	PaymentProviderStripe PaymentProvider = "stripe"
)

type PaymentStatus string

const (
	PaymentStatusCreated   PaymentStatus = "CREATED"
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusSucceeded PaymentStatus = "SUCCEEDED"
	PaymentStatusFailed    PaymentStatus = "FAILED"
	// PaymentStatusRefunded is appended to payment_status_history when a
	// Stripe refund (full or partial) completes. The `payments` row itself
	// stays at SUCCEEDED — we don't lose the fact that the charge succeeded.
	PaymentStatusRefunded PaymentStatus = "REFUNDED"
)

type Payment struct {
	ID       PaymentID
	OrderID  OrderID
	Provider PaymentProvider

	ProviderRef string
	Status      PaymentStatus

	RawPayload string

	CreatedAt time.Time
	UpdatedAt time.Time
}
