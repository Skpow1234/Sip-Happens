package domain

import "time"

// PaymentStatusHistoryEntry is one normalized status transition for an order's payment flow.
type PaymentStatusHistoryEntry struct {
	ID          int64
	OrderID     OrderID
	PaymentID   PaymentID
	Provider    PaymentProvider
	ProviderRef string
	Status      PaymentStatus
	Source      string
	Detail      string
	CreatedAt   time.Time
}
