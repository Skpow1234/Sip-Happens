package consumers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/events"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/workflows"
)

type fulfillmentStub struct {
	called domain.OrderID
}

func (f *fulfillmentStub) CreateShipment(ctx context.Context, _ time.Time, orderID domain.OrderID) (domain.Shipment, error) {
	_ = ctx
	f.called = orderID
	return domain.Shipment{OrderID: orderID}, nil
}

func TestOrderPaidHandler_Handle(t *testing.T) {
	stub := &fulfillmentStub{}
	saga := workflows.Saga{Fulfillment: stub}

	payload, _ := json.Marshal(events.OrderPaidV1{OrderID: "ord-1"})
	subj, err := events.SubjectForEventType("dev", events.TypeOrdersOrderPaid)
	if err != nil {
		t.Fatal(err)
	}
	msg := &nats.Msg{
		Subject: subj,
		Data:    payload,
		Header:  nats.Header{},
	}
	msg.Header.Set(outbox.NatsHeaderEventType, events.TypeOrdersOrderPaid)
	msg.Header.Set(outbox.NatsHeaderAggregateID, "ord-1")

	h := OrderPaidHandler{Namespace: "dev", Saga: saga}
	if err := h.Handle(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if stub.called != "ord-1" {
		t.Fatalf("expected ord-1, got %s", stub.called)
	}
}

func TestOrderPaidHandler_IgnoresOtherEventTypes(t *testing.T) {
	stub := &fulfillmentStub{}
	h := OrderPaidHandler{Namespace: "dev", Saga: workflows.Saga{Fulfillment: stub}}
	msg := &nats.Msg{
		Subject: "sip_happens.dev.events.orders.order.created",
		Header:  nats.Header{},
	}
	msg.Header.Set(outbox.NatsHeaderEventType, events.TypeOrdersOrderCreated)
	if err := h.Handle(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if stub.called != "" {
		t.Fatal("expected no fulfillment call")
	}
}
