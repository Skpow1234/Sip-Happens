package orders

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/services/core-api/internal/domain"
)

// Repository persists order state. All methods must be safe to call inside a transaction.
type Repository interface {
	Get(ctx context.Context, q Querier, id domain.OrderID) (*domain.Order, error)
	GetItems(ctx context.Context, q Querier, id domain.OrderID) ([]domain.OrderItem, error)

	Insert(ctx context.Context, tx *sql.Tx, order domain.Order, items []domain.OrderItem) error
	UpdateStatus(ctx context.Context, tx *sql.Tx, id domain.OrderID, status domain.OrderStatus) error

	SetStripeRefs(ctx context.Context, tx *sql.Tx, id domain.OrderID, checkoutSessionID, paymentIntentID string) error

	ListExpiredPending(ctx context.Context, q Querier, now time.Time, limit int) ([]domain.OrderID, error)
}

// Querier is implemented by *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

