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

