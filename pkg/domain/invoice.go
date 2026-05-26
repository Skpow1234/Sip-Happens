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

	// DIAN-specific fields populated by a real authorized provider (stub
	// provider leaves them empty). See migration 030.
	CUFE             string     // Código Único de Facturación Electrónica
	ResolutionNumber string     // DIAN resolución de autorización number
	ResolutionPrefix string     // resolution-scoped numbering prefix
	SequenceNumber   int64      // position within the resolution range
	XMLURL           string     // signed UBL XML stored at the provider or our object store
	PDFURL           string     // human-readable PDF receipt
	IssuedAt         *time.Time // moment DIAN acknowledged the document

	CreatedAt time.Time
	UpdatedAt time.Time
}
