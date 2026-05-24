package subscriptions

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

// ListPage is a paginated subscription list.
type ListPage struct {
	Subscriptions []domain.Subscription
	HasMore       bool
}

// UpdateInput is a partial update for PUT /subscriptions/{id}.
type UpdateInput struct {
	Qty       *int
	AddressID *domain.AddressID // nil = unchanged; empty AddressID clears
}

type Repository interface {
	ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit, offset int) (ListPage, error)
	Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.SubscriptionID) (*domain.Subscription, error)
	Create(ctx context.Context, q Querier, sub domain.Subscription) error
	Update(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.SubscriptionID, in UpdateInput) (*domain.Subscription, error)
	Cancel(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.SubscriptionID) (*domain.Subscription, error)
}
