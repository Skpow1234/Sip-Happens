package orders

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/events"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/outbox"
	"sip-happens/services/core-api/internal/payments"
)

type service struct {
	db        *db.DB
	orders    Repository
	inventory inventory.Repository
	catalog   catalog.Repository
	payments  payments.Repository
	outbox    outbox.Store
}

func NewService(d *db.DB, ordersRepo Repository, inventoryRepo inventory.Repository, catalogRepo catalog.Repository, paymentsRepo payments.Repository) Service {
	return NewServiceWithOutbox(d, ordersRepo, inventoryRepo, catalogRepo, paymentsRepo, nil)
}

func NewServiceWithOutbox(d *db.DB, ordersRepo Repository, inventoryRepo inventory.Repository, catalogRepo catalog.Repository, paymentsRepo payments.Repository, outboxStore outbox.Store) Service {
	return &service{
		db:        d,
		orders:    ordersRepo,
		inventory: inventoryRepo,
		catalog:   catalogRepo,
		payments:  paymentsRepo,
		outbox:    outboxStore,
	}
}

func (s *service) GetOrder(ctx context.Context, id domain.OrderID) (*domain.Order, []domain.OrderItem, error) {
	o, err := s.orders.Get(ctx, s.db.SQL, id)
	if err != nil {
		return nil, nil, err
	}
	items, err := s.orders.GetItems(ctx, s.db.SQL, id)
	if err != nil {
		return nil, nil, err
	}
	return o, items, nil
}

func (s *service) ListOrdersByCustomer(ctx context.Context, customerID domain.CustomerID, limit int) ([]domain.Order, error) {
	if customerID == "" {
		return nil, domain.ErrInvalidInput
	}
	return s.orders.ListByCustomer(ctx, s.db.SQL, customerID, limit)
}

func (s *service) CreatePendingOrder(ctx context.Context, in CreatePendingOrderInput) (*domain.Order, []domain.OrderItem, error) {
	var createdOrder *domain.Order
	var createdItems []domain.OrderItem

	err := s.db.WithTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, func(tx *sql.Tx) error {
		if s.catalog == nil {
			return domain.ErrInvalidInput
		}
		if in.Currency == "" {
			in.Currency = "COP"
		}

		now := time.Now()
		orderUUID, err := ids.UUIDv4()
		if err != nil {
			return err
		}
		orderID := domain.OrderID(orderUUID)

		// Price snapshot + product snapshot; reserve inventory after order row exists (FK).
		var subtotal int64
		items := make([]domain.OrderItem, 0, len(in.Items))
		type lineReserve struct {
			productID domain.ProductID
			qty       int
		}
		toReserve := make([]lineReserve, 0, len(in.Items))

		for _, it := range in.Items {
			if it.Qty <= 0 {
				return domain.ErrInvalidInput
			}

			p, err := s.catalog.GetProduct(ctx, tx, it.ProductID)
			if err != nil {
				return err
			}
			if !p.IsActive || p.InvimaReg == "" {
				return domain.ErrForbidden
			}

			price, err := s.catalog.GetCurrentPrice(ctx, tx, it.ProductID, in.Currency)
			if err != nil {
				return err
			}

			inv, err := s.inventory.LockForUpdate(ctx, tx, it.ProductID)
			if err != nil {
				return err
			}
			if inv.Available() < it.Qty {
				return domain.ErrConflict
			}

			unit := price.AmountMinorUnits
			line := unit * int64(it.Qty)
			subtotal += line

			items = append(items, domain.OrderItem{
				ProductID:           it.ProductID,
				SKU:                 p.SKU,
				Name:                p.Name,
				Qty:                 it.Qty,
				UnitPriceMinorUnits: unit,
				Currency:            in.Currency,
				LineTotalMinorUnits: line,
			})
			toReserve = append(toReserve, lineReserve{productID: it.ProductID, qty: it.Qty})
		}

		exp := now.Add(15 * time.Minute)
		ship := in.ShippingMinorUnits
		if ship < 0 {
			ship = 0
		}
		total := subtotal + ship
		order := domain.Order{
			ID:         orderID,
			Status:     domain.OrderStatusPendingPayment,
			CustomerID: in.CustomerID,

			SubtotalMinorUnits: subtotal,
			ShippingMinorUnits: ship,
			TotalMinorUnits:    total,
			Currency:           in.Currency,

			City:         in.City,
			Neighborhood: in.Neighborhood,
			AddressLine1: in.AddressLine1,
			Phone:        in.Phone,
			LegalID:      in.LegalID,
			FullName:     in.FullName,

			ExpiresAt: &exp,
			CreatedAt: now,
			UpdatedAt: now,
		}

		for i := range items {
			items[i].OrderID = order.ID
		}

		if err := s.orders.Insert(ctx, tx, order, items); err != nil {
			return err
		}

		for _, line := range toReserve {
			inv, err := s.inventory.LockForUpdate(ctx, tx, line.productID)
			if err != nil {
				return err
			}
			if inv.Available() < line.qty {
				return domain.ErrConflict
			}
			inv.Reserved += line.qty
			inv.LastModified = now
			if err := s.inventory.Update(ctx, tx, *inv); err != nil {
				return err
			}
			if err := s.inventory.CreateReservation(ctx, tx, orderID, line.productID, line.qty); err != nil {
				return err
			}
		}

		createdOrder = &order
		createdItems = items
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return createdOrder, createdItems, nil
}

func (s *service) CancelExpiredPendingOrders(ctx context.Context, now time.Time, limit int) (int, error) {
	ids, err := s.orders.ListExpiredPending(ctx, s.db.SQL, now, limit)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	updated := 0
	for _, id := range ids {
		err := s.db.WithTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, func(tx *sql.Tx) error {
			o, err := s.orders.Get(ctx, tx, id)
			if err != nil {
				return err
			}
			if o.Status != domain.OrderStatusPendingPayment {
				return nil
			}

			res, err := s.inventory.ListActiveReservations(ctx, tx, id)
			if err != nil {
				return err
			}
			for _, rr := range res {
				inv, err := s.inventory.LockForUpdate(ctx, tx, rr.ProductID)
				if err != nil {
					return err
				}
				inv.Reserved -= rr.Qty
				if inv.Reserved < 0 {
					inv.Reserved = 0
				}
				inv.LastModified = time.Now()
				if err := s.inventory.Update(ctx, tx, *inv); err != nil {
					return err
				}
				_ = s.inventory.MarkReservationReleased(ctx, tx, id, rr.ProductID)
			}

			return s.orders.UpdateStatus(ctx, tx, id, domain.OrderStatusCancelled)
		})
		if err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func (s *service) CancelPendingOrder(ctx context.Context, id domain.OrderID) error {
	return s.db.WithTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, func(tx *sql.Tx) error {
		o, err := s.orders.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if o.Status == domain.OrderStatusCancelled {
			return nil
		}
		if o.Status != domain.OrderStatusPendingPayment {
			return domain.ErrConflict
		}

		res, err := s.inventory.ListActiveReservations(ctx, tx, id)
		if err != nil {
			return err
		}
		for _, rr := range res {
			inv, err := s.inventory.LockForUpdate(ctx, tx, rr.ProductID)
			if err != nil {
				return err
			}
			inv.Reserved -= rr.Qty
			if inv.Reserved < 0 {
				inv.Reserved = 0
			}
			inv.LastModified = time.Now()
			if err := s.inventory.Update(ctx, tx, *inv); err != nil {
				return err
			}
			_ = s.inventory.MarkReservationReleased(ctx, tx, id, rr.ProductID)
		}

		return s.orders.UpdateStatus(ctx, tx, id, domain.OrderStatusCancelled)
	})
}

func (s *service) MarkPaid(ctx context.Context, id domain.OrderID, payment domain.Payment) error {
	return s.db.WithTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable}, func(tx *sql.Tx) error {
		o, err := s.orders.Get(ctx, tx, id)
		if err != nil {
			return err
		}
		if o.Status == domain.OrderStatusPaid {
			return domain.ErrAlreadyApplied
		}
		if o.Status != domain.OrderStatusPendingPayment {
			return domain.ErrConflict
		}

		items, err := s.orders.GetItems(ctx, tx, id)
		if err != nil {
			return err
		}
		// Prefer reservations as the source of truth for reserved qty.
		res, err := s.inventory.ListActiveReservations(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(res) == 0 {
			// fallback to order items
			for _, it := range items {
				res = append(res, inventory.Reservation{OrderID: id, ProductID: it.ProductID, Qty: it.Qty, Status: "ACTIVE"})
			}
		}

		for _, rr := range res {
			inv, err := s.inventory.LockForUpdate(ctx, tx, rr.ProductID)
			if err != nil {
				return err
			}
			if inv.Reserved < rr.Qty || inv.OnHand < rr.Qty {
				return domain.ErrConflict
			}
			inv.Reserved -= rr.Qty
			inv.OnHand -= rr.Qty
			inv.LastModified = time.Now()
			if err := s.inventory.Update(ctx, tx, *inv); err != nil {
				return err
			}
			_ = s.inventory.MarkReservationConsumed(ctx, tx, id, rr.ProductID)
		}

		paidAt := time.Now()
		var paymentID string
		if s.payments != nil {
			paymentUUID, err := ids.UUIDv4()
			if err != nil {
				return err
			}
			paymentID = paymentUUID
			payment.ID = domain.PaymentID(paymentUUID)
			payment.OrderID = id
			if payment.Provider == "" {
				payment.Provider = domain.PaymentProviderStripe
			}
			if payment.CreatedAt.IsZero() {
				payment.CreatedAt = paidAt
			}
			payment.UpdatedAt = paidAt
			if err := s.payments.Insert(ctx, tx, payment); err != nil {
				return err
			}
		}

		if err := s.orders.UpdateStatus(ctx, tx, id, domain.OrderStatusPaid); err != nil {
			return err
		}

		if s.outbox != nil {
			env, err := events.NewOrderPaidEnvelope(string(id), paymentID, "", paidAt)
			if err != nil {
				return err
			}
			ev, err := outbox.FromEnvelope(env)
			if err != nil {
				return err
			}
			if err := s.outbox.Insert(ctx, tx, ev); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *service) SetStripeRefs(ctx context.Context, id domain.OrderID, checkoutSessionID, paymentIntentID string) error {
	return s.db.WithTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(tx *sql.Tx) error {
		return s.orders.SetStripeRefs(ctx, tx, id, checkoutSessionID, paymentIntentID)
	})
}

