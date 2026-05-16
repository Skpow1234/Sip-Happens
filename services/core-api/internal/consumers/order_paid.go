package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/events"
	"sip-happens/services/core-api/internal/email"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/workflows"
)

// OrderPaidHandler starts fulfillment when orders.order.paid is consumed from JetStream.
type OrderPaidHandler struct {
	Namespace string
	Name      string // idempotency / metrics consumer name
	Saga      workflows.Saga
	Emails    *email.Service
}

func (h OrderPaidHandler) ConsumerName() string {
	if h.Name != "" {
		return h.Name
	}
	return "order-paid-fulfillment"
}

func (h OrderPaidHandler) Handle(ctx context.Context, m *nats.Msg) error {
	eventType := m.Header.Get(outbox.NatsHeaderEventType)
	if eventType == "" {
		parsed, err := events.ParseEventTypeFromSubject(h.Namespace, m.Subject)
		if err != nil {
			return err
		}
		eventType = parsed
	}
	if eventType != events.TypeOrdersOrderPaid {
		return nil
	}
	if err := events.ValidateEventSubject(h.Namespace, m.Subject, eventType); err != nil {
		return err
	}

	var payload events.OrderPaidV1
	if len(m.Data) > 0 {
		if err := json.Unmarshal(m.Data, &payload); err != nil {
			return fmt.Errorf("decode order.paid payload: %w", err)
		}
	}
	orderID := domain.OrderID(payload.OrderID)
	if orderID == "" {
		orderID = domain.OrderID(m.Header.Get(outbox.NatsHeaderAggregateID))
	}
	if orderID == "" {
		return fmt.Errorf("missing order_id in order.paid event")
	}

	if err := h.Saga.OnOrderPaid(ctx, time.Now().UTC(), orderID); err != nil {
		return err
	}
	if h.Emails != nil {
		if err := h.Emails.SendReceiptForOrder(ctx, orderID); err != nil {
			log.Printf("receipt email failed order_id=%s: %v", orderID, err)
		}
	}
	log.Printf("order.paid handled order_id=%s event_id=%s", orderID, m.Header.Get(outbox.NatsHeaderEventID))
	return nil
}
