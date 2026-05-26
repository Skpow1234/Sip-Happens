package domain

import "time"

type OrderID string

type OrderStatus string

const (
	OrderStatusDraft          OrderStatus = "DRAFT"
	OrderStatusPendingPayment OrderStatus = "PENDING_PAYMENT"
	OrderStatusPaid           OrderStatus = "PAID"
	OrderStatusFulfilling     OrderStatus = "FULFILLING"
	OrderStatusShipped        OrderStatus = "SHIPPED"
	OrderStatusCancelled      OrderStatus = "CANCELLED"
)

type Order struct {
	ID         OrderID
	DisplayRef string
	Status     OrderStatus
	CustomerID CustomerID

	SubtotalMinorUnits int64
	ShippingMinorUnits int64
	TotalMinorUnits    int64
	Currency           string

	City         string
	Neighborhood string
	AddressLine1 string
	Phone        string
	LegalID      string
	FullName     string

	StripeCheckoutSessionID string
	StripePaymentIntentID   string

	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrderItem struct {
	OrderID OrderID

	ProductID ProductID
	SKU       string
	Name      string
	Qty       int

	UnitPriceMinorUnits int64
	Currency            string
	LineTotalMinorUnits int64
}
