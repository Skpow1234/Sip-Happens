package events

import (
	"time"

	"sip-happens/pkg/ids"
)

const VersionOrdersOrderCreatedV1 = 1

type OrderCreatedV1 struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Currency   string `json:"currency"`
}

func NewOrderCreatedEnvelope(orderID, customerID, currency, requestID string, occurredAt time.Time) (Envelope, error) {
	if orderID == "" {
		return Envelope{}, ErrMissingOrderID
	}
	id, err := ids.UUIDv4()
	if err != nil {
		return Envelope{}, err
	}
	payload, err := JSON(OrderCreatedV1{
		OrderID:    orderID,
		CustomerID: customerID,
		Currency:   currency,
	})
	if err != nil {
		return Envelope{}, err
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return Envelope{
		ID:            id,
		Type:          TypeOrdersOrderCreated,
		OccurredAt:    occurredAt.UTC(),
		AggregateType: AggregateOrders,
		AggregateID:   orderID,
		Version:       VersionOrdersOrderCreatedV1,
		RequestID:     requestID,
		Payload:       payload,
	}, nil
}
