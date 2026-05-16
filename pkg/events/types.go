package events

// Domain event types (dot-separated, ADR 0006 / JetStream subject suffix).
const (
	TypeOrdersOrderPaid      = "orders.order.paid"
	TypeOrdersOrderCreated   = "orders.order.created"
	TypeOrdersOrderCancelled = "orders.order.cancelled"
)

const (
	AggregateOrders = "orders"
)
