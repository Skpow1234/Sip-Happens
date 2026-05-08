package payments

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) Insert(ctx context.Context, tx *sql.Tx, p domain.Payment) error {
	_, err := tx.ExecContext(ctx, `
insert into payments (id, order_id, provider, provider_ref, status, raw_payload, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8)
`, string(p.ID), string(p.OrderID), string(p.Provider), p.ProviderRef, p.Status, p.RawPayload, p.CreatedAt, p.UpdatedAt)
	if err == nil {
		return nil
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		// Idempotency: if the same provider_ref was already recorded for this order,
		// treat as no-op. If it belongs to a different order, surface a conflict.
		var existingOrderID string
		row := tx.QueryRowContext(ctx, `
select order_id
from payments
where provider=$1 and provider_ref=$2
`, string(p.Provider), p.ProviderRef)
		if scanErr := row.Scan(&existingOrderID); scanErr == nil {
			if existingOrderID == string(p.OrderID) {
				return nil
			}
			return domain.ErrConflict
		}
	}
	return err
}

