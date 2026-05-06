package inventory

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
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

