package events

import (
	"time"

	"sip-happens/pkg/ids"
)

// OrderPaidV1 is the v1 payload for orders.order.paid.
const VersionOrdersOrderPaidV1 = 1

type OrderPaidV1 struct {
	OrderID   string `json:"order_id"`
	PaymentID string `json:"payment_id,omitempty"`
}

func NewOrderPaidEnvelope(orderID, paymentID, requestID string, occurredAt time.Time) (Envelope, error) {
	if orderID == "" {
		return Envelope{}, ErrMissingOrderID
	}
	id, err := ids.UUIDv4()
	if err != nil {
		return Envelope{}, err
	}
	payload, err := JSON(OrderPaidV1{OrderID: orderID, PaymentID: paymentID})
	if err != nil {
		return Envelope{}, err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Envelope{
		ID:            id,
		Type:          TypeOrdersOrderPaid,
		OccurredAt:    occurredAt.UTC(),
		AggregateType: AggregateOrders,
		AggregateID:   orderID,
		Version:       VersionOrdersOrderPaidV1,
		RequestID:     requestID,
		Payload:       payload,
	}, nil
}
