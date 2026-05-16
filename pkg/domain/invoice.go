package domain

import "time"

type InvoiceStatus string

const (
	InvoiceStatusPending InvoiceStatus = "PENDING"
	InvoiceStatusIssued  InvoiceStatus = "ISSUED"
	InvoiceStatusFailed  InvoiceStatus = "FAILED"
)

type Invoice struct {
	ID          string
	OrderID     OrderID
	Provider    string
	ExternalID  string
	Status      InvoiceStatus
	PayloadJSON string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
