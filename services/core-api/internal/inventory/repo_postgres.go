package inventory

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
)

type PostgresRepository struct{}

func NewPostgresRepository() *PostgresRepository { return &PostgresRepository{} }

func (r *PostgresRepository) LockForUpdate(ctx context.Context, tx *sql.Tx, productID domain.ProductID) (*domain.Inventory, error) {
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

func (r *PostgresRepository) Update(ctx context.Context, tx *sql.Tx, inv domain.Inventory) error {
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

