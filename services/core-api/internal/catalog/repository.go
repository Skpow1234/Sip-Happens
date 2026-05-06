package catalog

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

type Repository interface {
	GetProduct(ctx context.Context, q Querier, id domain.ProductID) (*domain.Product, error)
	GetProductBySKU(ctx context.Context, q Querier, sku string) (*domain.Product, error)
	ListActiveProducts(ctx context.Context, q Querier, currency string, limit int) ([]ProductWithPrice, error)
	GetCurrentPrice(ctx context.Context, q Querier, productID domain.ProductID, currency string) (*domain.Price, error)
}

type ProductWithPrice struct {
	Product domain.Product
	Price   domain.Price
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

