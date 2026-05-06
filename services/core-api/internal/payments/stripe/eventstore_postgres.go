package stripe

import (
	"context"
	"database/sql"
	"time"
)

// SQLEventStore stores processed Stripe webhook event ids in a SQL database.
//
// This file currently uses PostgreSQL-compatible SQL, but the webhook handler
// depends only on the EventStore interface, so you can swap implementations.
type SQLEventStore struct {
	DB *sql.DB
}

func (s *SQLEventStore) HasProcessed(ctx context.Context, eventID string) (bool, error) {
	row := s.DB.QueryRowContext(ctx, `select 1 from stripe_webhook_events where event_id=$1`, eventID)
	var one int
	if err := row.Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *SQLEventStore) MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
insert into stripe_webhook_events (event_id, processed_at)
values ($1, $2)
on conflict (event_id) do nothing
`, eventID, processedAt)
	return err
}

