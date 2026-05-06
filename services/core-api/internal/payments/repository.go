package payments

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

type Repository interface {
	Insert(ctx context.Context, tx *sql.Tx, p domain.Payment) error
}

