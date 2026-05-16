package payments

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

// ExecContext is implemented by *sql.DB and *sql.Tx.
type ExecContext interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Repository interface {
	Insert(ctx context.Context, tx *sql.Tx, p domain.Payment) error
	RecordStatus(ctx context.Context, exec ExecContext, e domain.PaymentStatusHistoryEntry) error
	ListStatusHistoryForOrder(ctx context.Context, db *sql.DB, orderID domain.OrderID) ([]domain.PaymentStatusHistoryEntry, error)
}

