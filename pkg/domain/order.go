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
	// OrderStatusRefunded is set when a paid order has been fully or
	// partially refunded via POST /admin/orders/{id}/refund. Full vs
	// partial is distinguished by the rows in the `refunds` table — the
	// order status flips to REFUNDED on the FIRST refund regardless of
	// amount, because once any money has been sent back the order has
	// permanently left the normal lifecycle. Inventory is NOT
	// auto-restocked (a refunded order may still be in transit).
	OrderStatusRefunded OrderStatus = "REFUNDED"
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
