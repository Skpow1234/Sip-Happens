package workflows

import (
	"context"
	"time"

	"sip-happens/pkg/domain"
)

// This package is intentionally small: it defines workflow ports and a place to
// centralize saga steps without embedding transport or SQL details.

type FulfillmentStarter interface {
	CreateShipment(ctx context.Context, now time.Time, orderID domain.OrderID) error
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
	return s.Fulfillment.CreateShipment(ctx, now, orderID)
}

