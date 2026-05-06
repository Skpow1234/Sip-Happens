package catalog

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
)

type Repository interface {
	GetProduct(ctx context.Context, q Querier, id domain.ProductID) (*domain.Product, error)
	GetCurrentPrice(ctx context.Context, q Querier, productID domain.ProductID, currency string) (*domain.Price, error)
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

