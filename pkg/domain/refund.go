package domain

import "time"

type RefundID string

type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "PENDING"
	RefundStatusSucceeded RefundStatus = "SUCCEEDED"
	RefundStatusFailed    RefundStatus = "FAILED"
)

// Refund represents one row in the `refunds` table. A paid order can have
// multiple Refund rows over time — partial refunds are first-class. The
// order's status flips to REFUNDED on the first non-failed refund regardless
// of amount, because once any money goes back the order has permanently
// left the normal post-PAID lifecycle.
type Refund struct {
	ID                RefundID
	OrderID           OrderID
	PaymentID         PaymentID
	AmountMinorUnits  int64
	Currency          string
	Reason            string
	StripeRefundID    string
	IdempotencyKey    string
	Status            RefundStatus
	Actor             string
	FailureMessage    string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
