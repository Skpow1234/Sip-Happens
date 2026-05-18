package addresses

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Repository interface {
	ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID) ([]domain.CustomerAddress, error)
	Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.AddressID) (*domain.CustomerAddress, error)
	Create(ctx context.Context, q Querier, addr domain.CustomerAddress) error
	Delete(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.AddressID) error
	SetDefault(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.AddressID) error
}
