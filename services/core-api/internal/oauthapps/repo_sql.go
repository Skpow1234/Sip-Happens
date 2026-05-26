package oauthapps

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lib/pq"

	"sip-happens/pkg/domain"
)

type SQLClientRepository struct{}

func NewSQLClientRepository() *SQLClientRepository { return &SQLClientRepository{} }

const clientSelectCols = `
id, customer_id, name, client_id, client_secret_prefix, client_secret_hash,
redirect_uris, scopes, confidential, revoked_at, created_at, updated_at
`

func scanClient(row interface {
	Scan(dest ...any) error
}) (domain.OAuthClient, error) {
	var c domain.OAuthClient
	var redirects, scopes pq.StringArray
	var revoked sql.NullTime
	if err := row.Scan(
		&c.ID, &c.CustomerID, &c.Name, &c.ClientID, &c.ClientSecretPrefix, &c.ClientSecretHash,
		&redirects, &scopes, &c.Confidential, &revoked, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return domain.OAuthClient{}, err
	}
	c.RedirectURIs = []string(redirects)
	if len(scopes) > 0 {
		c.Scopes = make([]domain.PersonalAccessTokenScope, 0, len(scopes))
		for _, s := range scopes {
			c.Scopes = append(c.Scopes, domain.PersonalAccessTokenScope(s))
		}
	}
	if revoked.Valid {
		v := revoked.Time
		c.RevokedAt = &v
	}
	return c, nil
}

func (r *SQLClientRepository) GetByClientID(ctx context.Context, q Querier, clientID string) (*domain.OAuthClient, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, domain.ErrNotFound
	}
	row := q.QueryRowContext(ctx, `
select `+clientSelectCols+`
from oauth_clients
where client_id = $1 and revoked_at is null
`, clientID)
	c, err := scanClient(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// AllowsRedirect reports whether uri is in the registered RedirectURIs list.
// Comparison is byte-exact per RFC 6749 §3.1.2.
func AllowsRedirect(c *domain.OAuthClient, uri string) bool {
	if c == nil {
		return false
	}
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return false
	}
	for _, allowed := range c.RedirectURIs {
		if allowed == uri {
			return true
		}
	}
	return false
}

// ScopesContain reports whether `wanted` is a subset of the client's
// registered scope set (i.e. the third-party app cannot ask for more than
// the customer originally granted at registration).
func ScopesContain(client *domain.OAuthClient, wanted []domain.PersonalAccessTokenScope) bool {
	if client == nil {
		return false
	}
	allowed := make(map[domain.PersonalAccessTokenScope]struct{}, len(client.Scopes))
	for _, s := range client.Scopes {
		allowed[s] = struct{}{}
	}
	for _, w := range wanted {
		if _, ok := allowed[w]; !ok {
			return false
		}
	}
	return true
}
