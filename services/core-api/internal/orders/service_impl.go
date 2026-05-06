package orders

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/db"
	"sip-happens/services/core-api/internal/domain"
	"sip-happens/services/core-api/internal/inventory"
	"sip-happens/services/core-api/internal/payments"
)

type service struct {
	db        *db.DB
	orders    Repository
	inventory inventory.Repository
	catalog   catalog.Repository
	payments  payments.Repository
}

func NewService(d *db.DB, ordersRepo Repository, inventoryRepo inventory.Repository, catalogRepo catalog.Repository, paymentsRepo payments.Repository) Service {
	return &service{
		db:        d,
		orders:    ordersRepo,
		inventory: inventoryRepo,
		catalog:   catalogRepo,
		payments:  paymentsRepo,
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

		// Price snapshot + product snapshot + reservation.
		var subtotal int64
		items := make([]domain.OrderItem, 0, len(in.Items))

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

			inv.Reserved += it.Qty
			inv.LastModified = now
			if err := s.inventory.Update(ctx, tx, *inv); err != nil {
				return err
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
		}

		exp := now.Add(15 * time.Minute)
		orderUUID, err := ids.UUIDv4()
		if err != nil {
			return err
		}

		order := domain.Order{
			ID:         domain.OrderID(orderUUID),
			Status:     domain.OrderStatusPendingPayment,
			CustomerID: in.CustomerID,

			SubtotalMinorUnits: subtotal,
			ShippingMinorUnits: 0,
			TotalMinorUnits:    subtotal,
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

			items, err := s.orders.GetItems(ctx, tx, id)
			if err != nil {
				return err
			}

			for _, it := range items {
				inv, err := s.inventory.LockForUpdate(ctx, tx, it.ProductID)
				if err != nil {
					return err
				}
				inv.Reserved -= it.Qty
				if inv.Reserved < 0 {
					inv.Reserved = 0
				}
				inv.LastModified = time.Now()
				if err := s.inventory.Update(ctx, tx, *inv); err != nil {
					return err
				}
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

		for _, it := range items {
			inv, err := s.inventory.LockForUpdate(ctx, tx, it.ProductID)
			if err != nil {
				return err
			}
			if inv.Reserved < it.Qty || inv.OnHand < it.Qty {
				return domain.ErrConflict
			}
			inv.Reserved -= it.Qty
			inv.OnHand -= it.Qty
			inv.LastModified = time.Now()
			if err := s.inventory.Update(ctx, tx, *inv); err != nil {
				return err
			}
		}

		if s.payments != nil {
			paymentUUID, err := ids.UUIDv4()
			if err != nil {
				return err
			}
			payment.ID = domain.PaymentID(paymentUUID)
			payment.OrderID = id
			if payment.Provider == "" {
				payment.Provider = domain.PaymentProviderStripe
			}
			now := time.Now()
			if payment.CreatedAt.IsZero() {
				payment.CreatedAt = now
			}
			payment.UpdatedAt = now
			if err := s.payments.Insert(ctx, tx, payment); err != nil {
				return err
			}
		}

		return s.orders.UpdateStatus(ctx, tx, id, domain.OrderStatusPaid)
	})
}

