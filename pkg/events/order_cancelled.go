package events

import (
	"time"

	"sip-happens/pkg/ids"
)

const VersionOrdersOrderCancelledV1 = 1

type OrderCancelledV1 struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason,omitempty"`
}

func NewOrderCancelledEnvelope(orderID, reason, requestID string, occurredAt time.Time) (Envelope, error) {
	if orderID == "" {
		return Envelope{}, ErrMissingOrderID
	}
	id, err := ids.UUIDv4()
	if err != nil {
		return Envelope{}, err
	}
	payload, err := JSON(OrderCancelledV1{OrderID: orderID, Reason: reason})
	if err != nil {
		return Envelope{}, err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Envelope{
		ID:            id,
		Type:          TypeOrdersOrderCancelled,
		OccurredAt:    occurredAt.UTC(),
		AggregateType: AggregateOrders,
		AggregateID:   orderID,
		Version:       VersionOrdersOrderCancelledV1,
		RequestID:     requestID,
		Payload:       payload,
	}, nil
}
