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

// ClientRepository is the minimum surface needed by the OAuth flow handlers.
// Full CRUD (list/create/delete) belongs to the upcoming oauth-app-crud task.
type ClientRepository interface {
	// GetByClientID loads the active client row identified by its public
	// `client_id` string (NOT the internal UUID). Returns domain.ErrNotFound
	// when missing or revoked.
	GetByClientID(ctx context.Context, q Querier, clientID string) (*domain.OAuthClient, error)
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
