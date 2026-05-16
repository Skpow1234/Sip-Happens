package payments

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/redact"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }


func (r *SQLRepository) Insert(ctx context.Context, tx *sql.Tx, p domain.Payment) error {
	raw := p.RawPayload
	if raw != "" {
		raw = redact.JSON(raw)
	}
	_, err := tx.ExecContext(ctx, `
insert into payments (id, order_id, provider, provider_ref, status, raw_payload, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8)
`, string(p.ID), string(p.OrderID), string(p.Provider), p.ProviderRef, p.Status, raw, p.CreatedAt, p.UpdatedAt)
	if err == nil {
		return r.RecordStatus(ctx, tx, domain.PaymentStatusHistoryEntry{
			OrderID:     p.OrderID,
			PaymentID:   p.ID,
			Provider:    p.Provider,
			ProviderRef: p.ProviderRef,
			Status:      p.Status,
			Source:      "payment_recorded",
		})
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
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

func (r *SQLRepository) RecordStatus(ctx context.Context, exec ExecContext, e domain.PaymentStatusHistoryEntry) error {
	if exec == nil {
		return errors.New("missing db executor")
	}
	if e.OrderID == "" || e.Provider == "" || e.Status == "" || e.Source == "" {
		return domain.ErrInvalidInput
	}
	detail := e.Detail
	if detail != "" {
		detail = redact.JSON(detail)
	}
	var paymentID any
	if e.PaymentID != "" {
		paymentID = string(e.PaymentID)
	}
	_, err := exec.ExecContext(ctx, `
insert into payment_status_history (order_id, payment_id, provider, provider_ref, status, source, detail, created_at)
values ($1,$2,$3,$4,$5,$6,$7,coalesce($8, now()))
`, string(e.OrderID), paymentID, string(e.Provider), e.ProviderRef, e.Status, e.Source, nullableStr(detail), nullableTime(e.CreatedAt))
	return err
}

func (r *SQLRepository) ListStatusHistoryForOrder(ctx context.Context, db *sql.DB, orderID domain.OrderID) ([]domain.PaymentStatusHistoryEntry, error) {
	if db == nil {
		return nil, errors.New("missing db")
	}
	if orderID == "" {
		return nil, domain.ErrInvalidInput
	}
	rows, err := db.QueryContext(ctx, `
select id, order_id, payment_id, provider, provider_ref, status, source, coalesce(detail,''), created_at
from payment_status_history
where order_id=$1
order by created_at asc, id asc
`, string(orderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.PaymentStatusHistoryEntry
	for rows.Next() {
		var e domain.PaymentStatusHistoryEntry
		var paymentID sql.NullString
		if err := rows.Scan(
			&e.ID, &e.OrderID, &paymentID, &e.Provider, &e.ProviderRef, &e.Status, &e.Source, &e.Detail, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		if paymentID.Valid {
			e.PaymentID = domain.PaymentID(paymentID.String)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
