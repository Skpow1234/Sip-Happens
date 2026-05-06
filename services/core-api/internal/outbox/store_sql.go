package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SQLStore is a SQL database implementation of Store.
//
// This file currently uses PostgreSQL-compatible SQL, but callers depend on the
// Store interface, so you can swap implementations if needed.
type SQLStore struct{}

func NewSQLStore() *SQLStore { return &SQLStore{} }

func (s *SQLStore) Insert(ctx context.Context, tx *sql.Tx, e Event) error {
	if tx == nil {
		return fmt.Errorf("missing tx")
	}
	if e.ID == "" || e.Type == "" || e.AggregateType == "" || e.AggregateID == "" || e.PayloadJSON == "" {
		return fmt.Errorf("invalid outbox event")
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	if e.Version <= 0 {
		e.Version = 1
	}

	_, err := tx.ExecContext(ctx, `
insert into outbox_events (
  id, type, occurred_at,
  aggregate_type, aggregate_id, version,
  request_id, payload_json,
  status, attempts, next_attempt_at,
  created_at, updated_at
) values (
  $1,$2,$3,
  $4,$5,$6,
  $7,$8,
  'PENDING',0,now(),
  now(),now()
)
`, e.ID, e.Type, e.OccurredAt, e.AggregateType, e.AggregateID, e.Version, nullable(e.RequestID), e.PayloadJSON)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

