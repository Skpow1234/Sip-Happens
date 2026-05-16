package stripe

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"sip-happens/pkg/redact"
)

// DeadLetterRow is a stripe webhook dead-letter entry.
type DeadLetterRow struct {
	ID         int64
	EventID    string
	Reason     string
	RawPayload string
	CreatedAt  time.Time
}

func (s *SQLEventLogStore) ListDeadLetters(ctx context.Context, limit, offset int) ([]DeadLetterRow, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.DB.QueryContext(ctx, `
select id, coalesce(event_id,''), reason, coalesce(raw_payload,''), created_at
from stripe_dead_letters
order by created_at desc
limit $1 offset $2
`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeadLetterRow
	for rows.Next() {
		var r DeadLetterRow
		if err := rows.Scan(&r.ID, &r.EventID, &r.Reason, &r.RawPayload, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.RawPayload = redact.JSON(r.RawPayload)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *SQLEventLogStore) GetDeadLetterByID(ctx context.Context, id int64) (*DeadLetterRow, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	var r DeadLetterRow
	err := s.DB.QueryRowContext(ctx, `
select id, coalesce(event_id,''), reason, coalesce(raw_payload,''), created_at
from stripe_dead_letters where id=$1
`, id).Scan(&r.ID, &r.EventID, &r.Reason, &r.RawPayload, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.RawPayload = redact.JSON(r.RawPayload)
	return &r, nil
}

func (s *SQLEventLogStore) DeleteDeadLetter(ctx context.Context, id int64) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	_, err := s.DB.ExecContext(ctx, `delete from stripe_dead_letters where id=$1`, id)
	return err
}
