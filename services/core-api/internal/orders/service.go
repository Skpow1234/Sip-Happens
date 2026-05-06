package orders

import (
	"context"
	"time"

	"sip-happens/pkg/domain"
)

type Service interface {
	GetOrder(ctx context.Context, id domain.OrderID) (*domain.Order, []domain.OrderItem, error)

	// CreatePendingOrder reserves inventory and creates an order in PENDING_PAYMENT with an expiration.
	CreatePendingOrder(ctx context.Context, in CreatePendingOrderInput) (*domain.Order, []domain.OrderItem, error)

	// CancelExpiredPendingOrders releases reservations and cancels expired orders.
	CancelExpiredPendingOrders(ctx context.Context, now time.Time, limit int) (int, error)

	// MarkPaid transitions an order to PAID and applies the inventory move (reserved -> on_hand decrement).
	MarkPaid(ctx context.Context, id domain.OrderID, payment domain.Payment) error

	// SetStripeRefs stores Stripe references on an existing order.
	SetStripeRefs(ctx context.Context, id domain.OrderID, checkoutSessionID, paymentIntentID string) error
}

type CreatePendingOrderInput struct {
	CustomerID domain.CustomerID

	Currency string // default COP

	City         string
	Neighborhood string
	AddressLine1 string
	Phone        string
	LegalID      string
	FullName     string

	Items []CreatePendingOrderItem
}

type CreatePendingOrderItem struct {
	ProductID domain.ProductID
	Qty       int
}

