package payments

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) Insert(ctx context.Context, tx *sql.Tx, p domain.Payment) error {
	_, err := tx.ExecContext(ctx, `
insert into payments (id, order_id, provider, provider_ref, status, raw_payload, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8)
`, string(p.ID), string(p.OrderID), string(p.Provider), p.ProviderRef, p.Status, p.RawPayload, p.CreatedAt, p.UpdatedAt)
	return err
}

