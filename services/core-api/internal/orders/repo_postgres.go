package orders

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/pkg/domain"
)

type PostgresRepository struct{}

// SQLRepository is a SQL database implementation of Repository.
//
// This file currently uses PostgreSQL-compatible SQL, but the key point is that the
// *interface* (`orders.Repository`) is database-agnostic and can be re-implemented
// for a different database without changing domain/services.
type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) Get(ctx context.Context, q Querier, id domain.OrderID) (*domain.Order, error) {
	row := q.QueryRowContext(ctx, `
select id, display_ref, status, customer_id,
       subtotal_minor_units, shipping_minor_units, total_minor_units, currency,
       city, neighborhood, address_line1, phone, legal_id, full_name,
       stripe_checkout_session_id, stripe_payment_intent_id,
       expires_at, created_at, updated_at
from orders
where id = $1
`, string(id))

	var o domain.Order
	var expiresAt sql.NullTime
	if err := row.Scan(
		&o.ID, &o.DisplayRef, &o.Status, &o.CustomerID,
		&o.SubtotalMinorUnits, &o.ShippingMinorUnits, &o.TotalMinorUnits, &o.Currency,
		&o.City, &o.Neighborhood, &o.AddressLine1, &o.Phone, &o.LegalID, &o.FullName,
		&o.StripeCheckoutSessionID, &o.StripePaymentIntentID,
		&expiresAt, &o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		o.ExpiresAt = &t
	}
	return &o, nil
}

func (r *SQLRepository) GetItems(ctx context.Context, q Querier, id domain.OrderID) ([]domain.OrderItem, error) {
	rows, err := q.QueryContext(ctx, `
select order_id, product_id, sku, name, qty,
       unit_price_minor_units, currency, line_total_minor_units
from order_items
where order_id = $1
order by sku
`, string(id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OrderItem
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(
			&it.OrderID, &it.ProductID, &it.SKU, &it.Name, &it.Qty,
			&it.UnitPriceMinorUnits, &it.Currency, &it.LineTotalMinorUnits,
		); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (r *SQLRepository) Insert(ctx context.Context, tx *sql.Tx, order domain.Order, items []domain.OrderItem) error {
	// NOTE: Order ID generation strategy will be defined later (UUID/ULID).
	// For now, we assume the caller sets order.ID, or the DB provides it via default.
	_, err := tx.ExecContext(ctx, `
insert into orders (
  id, display_ref, status, customer_id,
  subtotal_minor_units, shipping_minor_units, total_minor_units, currency,
  city, neighborhood, address_line1, phone, legal_id, full_name,
  stripe_checkout_session_id, stripe_payment_intent_id,
  expires_at, created_at, updated_at
) values (
  $1, $2, $3, $4,
  $5, $6, $7, $8,
  $9, $10, $11, $12, $13, $14,
  $15, $16,
  $17, $18, $19
)
`, string(order.ID), order.DisplayRef, order.Status, string(order.CustomerID),
		order.SubtotalMinorUnits, order.ShippingMinorUnits, order.TotalMinorUnits, order.Currency,
		order.City, order.Neighborhood, order.AddressLine1, order.Phone, order.LegalID, order.FullName,
		order.StripeCheckoutSessionID, order.StripePaymentIntentID,
		order.ExpiresAt, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return err
	}

	for _, it := range items {
		_, err := tx.ExecContext(ctx, `
insert into order_items (
  order_id, product_id, sku, name, qty,
  unit_price_minor_units, currency, line_total_minor_units
) values ($1,$2,$3,$4,$5,$6,$7,$8)
`, string(it.OrderID), string(it.ProductID), it.SKU, it.Name, it.Qty, it.UnitPriceMinorUnits, it.Currency, it.LineTotalMinorUnits)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *SQLRepository) UpdateStatus(ctx context.Context, tx *sql.Tx, id domain.OrderID, status domain.OrderStatus) error {
	res, err := tx.ExecContext(ctx, `update orders set status=$1, updated_at=now() where id=$2`, status, string(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLRepository) SetStripeRefs(ctx context.Context, tx *sql.Tx, id domain.OrderID, checkoutSessionID, paymentIntentID string) error {
	res, err := tx.ExecContext(ctx, `
update orders
set stripe_checkout_session_id=$1,
    stripe_payment_intent_id=$2,
    updated_at=now()
where id=$3
`, checkoutSessionID, paymentIntentID, string(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLRepository) FindByStripeCheckoutSessionID(ctx context.Context, q Querier, checkoutSessionID string) (domain.OrderID, error) {
	row := q.QueryRowContext(ctx, `select id from orders where stripe_checkout_session_id=$1`, checkoutSessionID)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return domain.OrderID(id), nil
}

func (r *SQLRepository) FindByStripePaymentIntentID(ctx context.Context, q Querier, paymentIntentID string) (domain.OrderID, error) {
	row := q.QueryRowContext(ctx, `select id from orders where stripe_payment_intent_id=$1`, paymentIntentID)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return domain.OrderID(id), nil
}

func (r *SQLRepository) ListExpiredPending(ctx context.Context, q Querier, now time.Time, limit int) ([]domain.OrderID, error) {
	rows, err := q.QueryContext(ctx, `
select id
from orders
where status = 'PENDING_PAYMENT'
  and expires_at is not null
  and expires_at <= $1
order by expires_at asc
limit $2
`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OrderID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, domain.OrderID(id))
	}
	return out, rows.Err()
}

func (r *SQLRepository) ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit int) ([]domain.Order, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := q.QueryContext(ctx, `
select id, display_ref, status, customer_id,
       subtotal_minor_units, shipping_minor_units, total_minor_units, currency,
       city, neighborhood, address_line1, phone, legal_id, full_name,
       stripe_checkout_session_id, stripe_payment_intent_id,
       expires_at, created_at, updated_at
from orders
where customer_id = $1
order by created_at desc
limit $2
`, string(customerID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&o.ID, &o.DisplayRef, &o.Status, &o.CustomerID,
			&o.SubtotalMinorUnits, &o.ShippingMinorUnits, &o.TotalMinorUnits, &o.Currency,
			&o.City, &o.Neighborhood, &o.AddressLine1, &o.Phone, &o.LegalID, &o.FullName,
			&o.StripeCheckoutSessionID, &o.StripePaymentIntentID,
			&expiresAt, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			o.ExpiresAt = &t
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
