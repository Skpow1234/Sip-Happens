package oauthapps

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

// ClientPage is one page of a customer-scoped client listing.
type ClientPage struct {
	Clients []domain.OAuthClient
	HasMore bool
}

// ClientRepository persists registered third-party OAuth apps.
//
// All customer-scoped methods MUST verify ownership (customer_id) so a caller
// can never read or mutate another customer's client rows. GetByClientID is
// the only intentionally unscoped lookup — it backs the public POST /oauth/token
// endpoint and is gated by client_secret verification afterwards.
type ClientRepository interface {
	GetByClientID(ctx context.Context, q Querier, clientID string) (*domain.OAuthClient, error)

	ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit, offset int) (ClientPage, error)
	Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.OAuthClientID) (*domain.OAuthClient, error)
	Create(ctx context.Context, q Querier, c domain.OAuthClient) error
	Revoke(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.OAuthClientID) (*domain.OAuthClient, error)
}

// CodeRepository persists short-lived authorization codes for the
// Authorization Code grant. Codes are single-use: Consume must succeed at
// most once per row, even under concurrent /oauth/token attempts.
type CodeRepository interface {
	Create(ctx context.Context, q Querier, c domain.OAuthAuthorizationCode) error
	// Consume atomically marks the row identified by codePrefix as used and
	// returns it. Returns domain.ErrNotFound when the prefix is unknown,
	// already consumed, or expired.
	Consume(ctx context.Context, q Querier, codePrefix string) (*domain.OAuthAuthorizationCode, error)
}
