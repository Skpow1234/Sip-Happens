package oauthapps

import (
	"context"
	"database/sql"
	"strings"
	"time"

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

func (r *SQLClientRepository) ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit, offset int) (ClientPage, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	// Fetch limit+1 to compute has_more in a single round-trip.
	rows, err := q.QueryContext(ctx, `
select `+clientSelectCols+`
from oauth_clients
where customer_id = $1
order by created_at desc, id desc
limit $2 offset $3
`, string(customerID), limit+1, offset)
	if err != nil {
		return ClientPage{}, err
	}
	defer rows.Close()

	out := make([]domain.OAuthClient, 0, limit)
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return ClientPage{}, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return ClientPage{}, err
	}
	page := ClientPage{}
	if len(out) > limit {
		page.HasMore = true
		out = out[:limit]
	}
	page.Clients = out
	return page, nil
}

func (r *SQLClientRepository) Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.OAuthClientID) (*domain.OAuthClient, error) {
	idStr := strings.TrimSpace(string(id))
	if idStr == "" {
		return nil, domain.ErrNotFound
	}
	row := q.QueryRowContext(ctx, `
select `+clientSelectCols+`
from oauth_clients
where id = $1 and customer_id = $2
`, idStr, string(customerID))
	c, err := scanClient(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *SQLClientRepository) Create(ctx context.Context, q Querier, c domain.OAuthClient) error {
	redirects := make(pq.StringArray, 0, len(c.RedirectURIs))
	for _, u := range c.RedirectURIs {
		redirects = append(redirects, u)
	}
	scopes := make(pq.StringArray, 0, len(c.Scopes))
	for _, s := range c.Scopes {
		scopes = append(scopes, string(s))
	}
	_, err := q.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris, scopes, confidential,
  created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`,
		string(c.ID), string(c.CustomerID), c.Name,
		c.ClientID, c.ClientSecretPrefix, c.ClientSecretHash,
		redirects, scopes, c.Confidential,
		c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// Revoke is idempotent: an already-revoked row returns its stored snapshot
// with the existing revoked_at preserved. Returns ErrNotFound only when no
// row matches the (customer, id) pair.
func (r *SQLClientRepository) Revoke(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.OAuthClientID) (*domain.OAuthClient, error) {
	idStr := strings.TrimSpace(string(id))
	if idStr == "" {
		return nil, domain.ErrNotFound
	}
	now := time.Now()
	row := q.QueryRowContext(ctx, `
update oauth_clients
set revoked_at = coalesce(revoked_at, $3),
    updated_at = case when revoked_at is null then $3 else updated_at end
where id = $1 and customer_id = $2
returning `+clientSelectCols, idStr, string(customerID), now)
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
