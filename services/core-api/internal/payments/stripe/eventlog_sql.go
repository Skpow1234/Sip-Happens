package stripe

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/pkg/redact"
)

type EventLogStore interface {
	UpsertReceived(ctx context.Context, eventID, eventType, orderID, raw string, receivedAt time.Time) error
	MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error
	MarkIgnored(ctx context.Context, eventID string, processedAt time.Time) error
	MarkFailed(ctx context.Context, eventID string, processedAt time.Time, errMsg string) error
	InsertDeadLetter(ctx context.Context, eventID, reason, raw string) error
}

type SQLEventLogStore struct {
	DB *sql.DB
}

func (s *SQLEventLogStore) UpsertReceived(ctx context.Context, eventID, eventType, orderID, raw string, receivedAt time.Time) error {
	raw = redact.JSON(raw)
	_, err := s.DB.ExecContext(ctx, `
insert into stripe_event_log (event_id, event_type, order_id, received_at, status, raw_payload)
values ($1,$2,$3,$4,'RECEIVED',$5)
on conflict (event_id) do update
set event_type=excluded.event_type,
    order_id=coalesce(excluded.order_id, stripe_event_log.order_id),
    raw_payload=excluded.raw_payload
`, eventID, eventType, nullable(orderID), receivedAt, raw)
	return err
}

func (s *SQLEventLogStore) MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
update stripe_event_log
set status='PROCESSED', processed_at=$2, last_error=null
where event_id=$1
`, eventID, processedAt)
	return err
}

func (s *SQLEventLogStore) MarkIgnored(ctx context.Context, eventID string, processedAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
update stripe_event_log
set status='IGNORED', processed_at=$2, last_error=null
where event_id=$1
`, eventID, processedAt)
	return err
}

func (s *SQLEventLogStore) MarkFailed(ctx context.Context, eventID string, processedAt time.Time, errMsg string) error {
	errMsg = redact.String(errMsg)
	_, err := s.DB.ExecContext(ctx, `
update stripe_event_log
set status='FAILED', processed_at=$2, last_error=$3
where event_id=$1
`, eventID, processedAt, errMsg)
	return err
}

func (s *SQLEventLogStore) InsertDeadLetter(ctx context.Context, eventID, reason, raw string) error {
	raw = redact.JSON(raw)
	_, err := s.DB.ExecContext(ctx, `
insert into stripe_dead_letters (event_id, reason, raw_payload)
values ($1,$2,$3)
`, nullable(eventID), reason, raw)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
