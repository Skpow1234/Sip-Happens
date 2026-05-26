package events

import (
	"time"

	"sip-happens/pkg/ids"
)

const VersionOrdersOrderStatusOverriddenV1 = 1

// OrderStatusOverriddenV1 is emitted when an admin forcibly transitions an
// order via POST /admin/orders/{id}/status, bypassing the normal lifecycle.
// Consumers should treat this as a signal to reconcile (e.g. an emergency
// SHIPPED override should retroactively trigger any shipment-side actions
// that would normally have fired on the shipment workflow).
type OrderStatusOverriddenV1 struct {
	OrderID    string `json:"order_id"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Actor      string `json:"actor"`
	Reason     string `json:"reason"`
}

func NewOrderStatusOverriddenEnvelope(orderID, fromStatus, toStatus, actor, reason, requestID string, occurredAt time.Time) (Envelope, error) {
	if orderID == "" {
		return Envelope{}, ErrMissingOrderID
	}
	id, err := ids.UUIDv4()
	if err != nil {
		return Envelope{}, err
	}
	payload, err := JSON(OrderStatusOverriddenV1{
		OrderID:    orderID,
		FromStatus: fromStatus,
		ToStatus:   toStatus,
		Actor:      actor,
		Reason:     reason,
	})
	if err != nil {
		return Envelope{}, err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Envelope{
		ID:            id,
		Type:          TypeOrdersOrderStatusOverridden,
		OccurredAt:    occurredAt.UTC(),
		AggregateType: AggregateOrders,
		AggregateID:   orderID,
		Version:       VersionOrdersOrderStatusOverriddenV1,
		RequestID:     requestID,
		Payload:       payload,
	}, nil
}
