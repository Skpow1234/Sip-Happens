package inventory

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

// SQLRepository is a SQL database implementation of inventory.Repository.
//
// This file currently uses PostgreSQL-compatible SQL, but the interface remains
// database-agnostic and can be re-implemented for a different database.
type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) LockForUpdate(ctx context.Context, tx *sql.Tx, productID domain.ProductID) (*domain.Inventory, error) {
	row := tx.QueryRowContext(ctx, `
select product_id, on_hand, reserved, safety_stock, updated_at
from inventory
where product_id = $1
for update
`, string(productID))

	var inv domain.Inventory
	if err := row.Scan(&inv.ProductID, &inv.OnHand, &inv.Reserved, &inv.SafetyStock, &inv.LastModified); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &inv, nil
}

func (r *SQLRepository) Update(ctx context.Context, tx *sql.Tx, inv domain.Inventory) error {
	res, err := tx.ExecContext(ctx, `
update inventory
set on_hand=$1, reserved=$2, safety_stock=$3, updated_at=now()
where product_id=$4
`, inv.OnHand, inv.Reserved, inv.SafetyStock, string(inv.ProductID))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLRepository) CreateReservation(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID, qty int) error {
	if qty <= 0 {
		return domain.ErrInvalidInput
	}
	_, err := tx.ExecContext(ctx, `
insert into inventory_reservations (order_id, product_id, qty, status, created_at, updated_at)
values ($1,$2,$3,'ACTIVE',now(),now())
on conflict (order_id, product_id) do update
set qty=excluded.qty,
    status='ACTIVE',
    updated_at=now()
`, string(orderID), string(productID), qty)
	return err
}

func (r *SQLRepository) ListActiveReservations(ctx context.Context, tx *sql.Tx, orderID domain.OrderID) ([]Reservation, error) {
	rows, err := tx.QueryContext(ctx, `
select order_id, product_id, qty, status
from inventory_reservations
where order_id=$1 and status='ACTIVE'
order by product_id
`, string(orderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Reservation
	for rows.Next() {
		var rr Reservation
		if err := rows.Scan(&rr.OrderID, &rr.ProductID, &rr.Qty, &rr.Status); err != nil {
			return nil, err
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *SQLRepository) MarkReservationReleased(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID) error {
	_, err := tx.ExecContext(ctx, `
update inventory_reservations
set status='RELEASED', updated_at=now()
where order_id=$1 and product_id=$2 and status='ACTIVE'
`, string(orderID), string(productID))
	return err
}

func (r *SQLRepository) MarkReservationConsumed(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID) error {
	_, err := tx.ExecContext(ctx, `
update inventory_reservations
set status='CONSUMED', updated_at=now()
where order_id=$1 and product_id=$2 and status='ACTIVE'
`, string(orderID), string(productID))
	return err
}

