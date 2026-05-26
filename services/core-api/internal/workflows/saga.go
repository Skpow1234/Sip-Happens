package workflows

import (
	"context"
	"time"

	"sip-happens/pkg/domain"
)

// This package is intentionally small: it defines workflow ports and a place to
// centralize saga steps without embedding transport or SQL details.

type FulfillmentStarter interface {
	StartFulfillment(ctx context.Context, now time.Time, orderID domain.OrderID) (domain.Shipment, error)
}

type Saga struct {
	Fulfillment FulfillmentStarter
}

// OnOrderPaid is the handoff point from "paid" to "fulfillment started".
// It is designed to be called by a consumer of `orders.order.paid` events.
func (s *Saga) OnOrderPaid(ctx context.Context, now time.Time, orderID domain.OrderID) error {
	if s == nil || s.Fulfillment == nil {
		return nil
	}
	_, err := s.Fulfillment.StartFulfillment(ctx, now, orderID)
	return err
}
