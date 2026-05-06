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

func (s *SQLStore) ClaimPending(ctx context.Context, db *sql.DB, now time.Time, limit int, lockedBy string) ([]Event, error) {
	if db == nil {
		return nil, fmt.Errorf("missing db")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if lockedBy == "" {
		lockedBy = "outbox-dispatcher"
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
select id, type, occurred_at, aggregate_type, aggregate_id, version,
       coalesce(request_id,''), payload_json
from outbox_events
where status='PENDING'
  and next_attempt_at <= $1
  and (locked_at is null or locked_at < $1 - interval '5 minutes')
order by next_attempt_at asc, occurred_at asc
for update skip locked
limit $2
`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.OccurredAt, &e.AggregateType, &e.AggregateID, &e.Version, &e.RequestID, &e.PayloadJSON); err != nil {
			return nil, err
		}
		ids = append(ids, e.ID)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Lock metadata update.
	_, err = tx.ExecContext(ctx, `
update outbox_events
set locked_at=$2,
    locked_by=$3,
    updated_at=now()
where id = any($1)
`, pqStringArray(ids), now, lockedBy)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SQLStore) MarkSent(ctx context.Context, db *sql.DB, id string, sentAt time.Time) error {
	if db == nil {
		return fmt.Errorf("missing db")
	}
	if id == "" {
		return fmt.Errorf("missing id")
	}
	_, err := db.ExecContext(ctx, `
update outbox_events
set status='SENT',
    locked_at=null,
    locked_by=null,
    updated_at=$2
where id=$1
`, id, sentAt)
	return err
}

func (s *SQLStore) MarkFailed(ctx context.Context, db *sql.DB, id string, now time.Time, errMsg string, nextAttemptAt time.Time) error {
	if db == nil {
		return fmt.Errorf("missing db")
	}
	if id == "" {
		return fmt.Errorf("missing id")
	}
	if nextAttemptAt.IsZero() {
		nextAttemptAt = now.Add(30 * time.Second)
	}
	_, err := db.ExecContext(ctx, `
update outbox_events
set status='PENDING',
    attempts=attempts+1,
    next_attempt_at=$2,
    locked_at=null,
    locked_by=null,
    updated_at=$3
where id=$1
`, id, nextAttemptAt, now)
	_ = errMsg
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// pqStringArray formats a slice for Postgres ANY($1) usage without adding new dependencies.
// It relies on lib/pq's array support via []string.
func pqStringArray(v []string) any { return v }

