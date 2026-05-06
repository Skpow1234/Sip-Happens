package inventory

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
)

type Repository interface {
	// LockForUpdate must SELECT ... FOR UPDATE to protect reservation integrity.
	LockForUpdate(ctx context.Context, tx *sql.Tx, productID domain.ProductID) (*domain.Inventory, error)

	// Update writes updated inventory values.
	Update(ctx context.Context, tx *sql.Tx, inv domain.Inventory) error
}

