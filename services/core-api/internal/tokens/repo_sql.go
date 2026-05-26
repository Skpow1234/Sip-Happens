package tokens

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lib/pq"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

const tokenSelectCols = `
id, customer_id, name, token_prefix, token_hash, scopes,
last_used_at, expires_at, revoked_at, created_at, updated_at
`

func scanToken(row interface {
	Scan(dest ...any) error
}) (domain.PersonalAccessToken, error) {
	var t domain.PersonalAccessToken
	var scopes pq.StringArray
	var lastUsed, expires, revoked sql.NullTime
	if err := row.Scan(
		&t.ID, &t.CustomerID, &t.Name, &t.TokenPrefix, &t.TokenHash, &scopes,
		&lastUsed, &expires, &revoked, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return domain.PersonalAccessToken{}, err
	}
	if len(scopes) > 0 {
		t.Scopes = make([]domain.PersonalAccessTokenScope, 0, len(scopes))
		for _, s := range scopes {
			t.Scopes = append(t.Scopes, domain.PersonalAccessTokenScope(s))
		}
	}
	if lastUsed.Valid {
		v := lastUsed.Time
		t.LastUsedAt = &v
	}
	if expires.Valid {
		v := expires.Time
		t.ExpiresAt = &v
	}
	if revoked.Valid {
		v := revoked.Time
		t.RevokedAt = &v
	}
	return t, nil
}

func (r *SQLRepository) ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit, offset int) (ListPage, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := q.QueryContext(ctx, `
select `+tokenSelectCols+`
from personal_access_tokens
where customer_id = $1
order by created_at desc
limit $2 offset $3
`, string(customerID), limit+1, offset)
	if err != nil {
		return ListPage{}, err
	}
	defer rows.Close()

	var out []domain.PersonalAccessToken
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return ListPage{}, err
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return ListPage{}, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return ListPage{Tokens: out, HasMore: hasMore}, nil
}

func (r *SQLRepository) Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.PersonalAccessTokenID) (*domain.PersonalAccessToken, error) {
	row := q.QueryRowContext(ctx, `
select `+tokenSelectCols+`
from personal_access_tokens
where customer_id = $1 and id = $2
`, string(customerID), string(id))
	t, err := scanToken(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *SQLRepository) Create(ctx context.Context, q Querier, t domain.PersonalAccessToken) error {
	scopes := make(pq.StringArray, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, string(s))
	}
	var expires any
	if t.ExpiresAt != nil {
		expires = *t.ExpiresAt
	}
	_, err := q.ExecContext(ctx, `
insert into personal_access_tokens (
  id, customer_id, name, token_prefix, token_hash, scopes,
  expires_at, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
`, string(t.ID), string(t.CustomerID), strings.TrimSpace(t.Name), t.TokenPrefix, t.TokenHash, scopes,
		expires, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *SQLRepository) Revoke(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.PersonalAccessTokenID) (*domain.PersonalAccessToken, error) {
	cur, err := r.Get(ctx, q, customerID, id)
	if err != nil {
		return nil, err
	}
	if cur.RevokedAt != nil {
		return cur, nil
	}
	now := time.Now()
	res, err := q.ExecContext(ctx, `
update personal_access_tokens
set revoked_at = $3, updated_at = now()
where customer_id = $1 and id = $2 and revoked_at is null
`, string(customerID), string(id), now)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, domain.ErrNotFound
	}
	return r.Get(ctx, q, customerID, id)
}

func (r *SQLRepository) GetByPrefix(ctx context.Context, q Querier, prefix string) (*domain.PersonalAccessToken, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, domain.ErrNotFound
	}
	row := q.QueryRowContext(ctx, `
select `+tokenSelectCols+`
from personal_access_tokens
where token_prefix = $1 and revoked_at is null
`, prefix)
	t, err := scanToken(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if !t.IsActive(time.Now()) {
		return nil, domain.ErrNotFound
	}
	return &t, nil
}

func (r *SQLRepository) TouchLastUsed(ctx context.Context, q Querier, prefix string) error {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}
	_, err := q.ExecContext(ctx, `
update personal_access_tokens
set last_used_at = now()
where token_prefix = $1 and revoked_at is null
`, prefix)
	return err
}

// ResolveToken loads a PAT by public id or internal UUID for a given customer.
func ResolveToken(ctx context.Context, repo Repository, q Querier, customerID domain.CustomerID, ref string) (*domain.PersonalAccessToken, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, domain.ErrInvalidInput
	}
	if id, ok := IDFromPublicID(ref); ok {
		return repo.Get(ctx, q, customerID, id)
	}
	return repo.Get(ctx, q, customerID, domain.PersonalAccessTokenID(ref))
}
