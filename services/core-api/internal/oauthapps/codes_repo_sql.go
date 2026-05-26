package oauthapps

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lib/pq"

	"sip-happens/pkg/domain"
)

type SQLCodeRepository struct{}

func NewSQLCodeRepository() *SQLCodeRepository { return &SQLCodeRepository{} }

const codeSelectCols = `
id, oauth_client_id, customer_id, code_prefix, code_hash,
redirect_uri, scopes, expires_at, consumed_at, created_at
`

func scanCode(row interface {
	Scan(dest ...any) error
}) (domain.OAuthAuthorizationCode, error) {
	var c domain.OAuthAuthorizationCode
	var scopes pq.StringArray
	var consumed sql.NullTime
	if err := row.Scan(
		&c.ID, &c.OAuthClientID, &c.CustomerID, &c.CodePrefix, &c.CodeHash,
		&c.RedirectURI, &scopes, &c.ExpiresAt, &consumed, &c.CreatedAt,
	); err != nil {
		return domain.OAuthAuthorizationCode{}, err
	}
	if len(scopes) > 0 {
		c.Scopes = make([]domain.PersonalAccessTokenScope, 0, len(scopes))
		for _, s := range scopes {
			c.Scopes = append(c.Scopes, domain.PersonalAccessTokenScope(s))
		}
	}
	if consumed.Valid {
		v := consumed.Time
		c.ConsumedAt = &v
	}
	return c, nil
}

func (r *SQLCodeRepository) Create(ctx context.Context, q Querier, c domain.OAuthAuthorizationCode) error {
	scopes := make(pq.StringArray, 0, len(c.Scopes))
	for _, s := range c.Scopes {
		scopes = append(scopes, string(s))
	}
	_, err := q.ExecContext(ctx, `
insert into oauth_authorization_codes (
  id, oauth_client_id, customer_id, code_prefix, code_hash,
  redirect_uri, scopes, expires_at, created_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
`,
		string(c.ID), string(c.OAuthClientID), string(c.CustomerID),
		c.CodePrefix, c.CodeHash, strings.TrimSpace(c.RedirectURI),
		scopes, c.ExpiresAt, c.CreatedAt,
	)
	return err
}

// Consume atomically marks the matching row as used and returns its snapshot.
// Concurrent /oauth/token calls for the same code race here — only the first
// returns the row; subsequent attempts get domain.ErrNotFound.
func (r *SQLCodeRepository) Consume(ctx context.Context, q Querier, codePrefix string) (*domain.OAuthAuthorizationCode, error) {
	codePrefix = strings.TrimSpace(codePrefix)
	if codePrefix == "" {
		return nil, domain.ErrNotFound
	}
	row := q.QueryRowContext(ctx, `
update oauth_authorization_codes
set consumed_at = $2
where code_prefix = $1
  and consumed_at is null
  and expires_at > now()
returning `+codeSelectCols, codePrefix, time.Now())
	c, err := scanCode(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}
