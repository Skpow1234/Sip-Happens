package payments

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
)

type Repository interface {
	Insert(ctx context.Context, tx *sql.Tx, p domain.Payment) error
}

