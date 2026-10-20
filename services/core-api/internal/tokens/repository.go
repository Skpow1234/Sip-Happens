package tokens

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

// ListPage is a paginated PAT list (newest first).
type ListPage struct {
	Tokens  []domain.PersonalAccessToken
	HasMore bool
}

// Repository abstracts personal_access_tokens persistence.
type Repository interface {
	ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit, offset int) (ListPage, error)
	Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.PersonalAccessTokenID) (*domain.PersonalAccessToken, error)
	Create(ctx context.Context, q Querier, t domain.PersonalAccessToken) error
	Revoke(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.PersonalAccessTokenID) (*domain.PersonalAccessToken, error)
	// GetByPrefix loads an active token by its lookup prefix for Bearer-PAT
	// auth middleware. Returns ErrNotFound when missing or already
	// revoked/expired.
	GetByPrefix(ctx context.Context, q Querier, prefix string) (*domain.PersonalAccessToken, error)
	// TouchLastUsed records token.last_used_at = now for the row identified by
	// prefix. Best-effort; the auth path should not fail when this errors.
	TouchLastUsed(ctx context.Context, q Querier, prefix string) error
}
