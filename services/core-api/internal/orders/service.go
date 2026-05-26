package orders

import (
	"context"
	"time"

	"sip-happens/pkg/domain"
)

type Service interface {
	GetOrder(ctx context.Context, id domain.OrderID) (*domain.Order, []domain.OrderItem, error)

	// GetOrderForCustomer loads an order by internal id or display_ref (SH-…) scoped to the customer.
	GetOrderForCustomer(ctx context.Context, customerID domain.CustomerID, ref string) (*domain.Order, []domain.OrderItem, error)

	// ListOrdersByCustomer returns recent orders for a customer (newest first) with pagination.
	ListOrdersByCustomer(ctx context.Context, customerID domain.CustomerID, limit, offset int) (CustomerOrdersPage, error)

	// CreatePendingOrder reserves inventory and creates an order in PENDING_PAYMENT with an expiration.
	CreatePendingOrder(ctx context.Context, in CreatePendingOrderInput) (*domain.Order, []domain.OrderItem, error)

	// CancelExpiredPendingOrders releases reservations and cancels expired orders.
	CancelExpiredPendingOrders(ctx context.Context, now time.Time, limit int) (int, error)

	// MarkPaid transitions an order to PAID and applies the inventory move (reserved -> on_hand decrement).
	MarkPaid(ctx context.Context, id domain.OrderID, payment domain.Payment) error

	// CancelPendingOrder releases active reservations and sets status CANCELLED
	// (idempotent if already cancelled). The reason is recorded on the
	// inventory release audit row and on the order.cancelled outbox envelope;
	// callers should pass a short stable code like "customer_cancel",
	// "admin_cancel", "expired" or "stripe_session_expired". Empty reason
	// defaults to "cancelled" for backwards compatibility.
	CancelPendingOrder(ctx context.Context, id domain.OrderID, reason string) error

	// SetStripeRefs stores Stripe references on an existing order.
	SetStripeRefs(ctx context.Context, id domain.OrderID, checkoutSessionID, paymentIntentID string) error

	// AdminOverrideStatus forcibly transitions an order from its current
	// status to `to`, bypassing the normal lifecycle (NATS event, Stripe
	// webhook, shipment workflow). Intended as an emergency escape hatch
	// for support/ops when the automated path is broken (e.g. NATS down,
	// shipment provider offline). The (from, to) pair must be in
	// AdminAllowedTransitions or the call returns ErrConflict. Inventory
	// reservations are NOT released when forcing to CANCELLED from a
	// post-PAID state — operators must run a refund separately.
	// `reason` is required (non-empty) and is recorded on the
	// `order.status.overridden` outbox envelope; the audit row is
	// written by the HTTP handler so the actor header lives at the edge.
	AdminOverrideStatus(ctx context.Context, id domain.OrderID, to domain.OrderStatus, reason string) (*domain.Order, error)
}

// AdminAllowedTransitions is the whitelist enforced by AdminOverrideStatus.
// Only forward moves through the lifecycle plus any-state→CANCELLED are
// permitted. Backward transitions (e.g. SHIPPED → PAID) and skip-ahead
// transitions (e.g. PENDING_PAYMENT → SHIPPED) are forbidden because they
// invariably leave inventory, payments and shipment state inconsistent.
var AdminAllowedTransitions = map[domain.OrderStatus][]domain.OrderStatus{
	domain.OrderStatusPendingPayment: {domain.OrderStatusPaid, domain.OrderStatusCancelled},
	domain.OrderStatusPaid:           {domain.OrderStatusFulfilling, domain.OrderStatusCancelled},
	domain.OrderStatusFulfilling:     {domain.OrderStatusShipped, domain.OrderStatusCancelled},
	domain.OrderStatusShipped:        {domain.OrderStatusCancelled},
}

// IsAdminAllowedTransition reports whether AdminOverrideStatus accepts
// the (from, to) pair. CANCELLED → anything is rejected because CANCELLED
// is terminal.
func IsAdminAllowedTransition(from, to domain.OrderStatus) bool {
	if from == to {
		return false
	}
	allowed, ok := AdminAllowedTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
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

	ShippingMinorUnits int64

	Items []CreatePendingOrderItem
}

type CreatePendingOrderItem struct {
	ProductID domain.ProductID
	Qty       int
}
