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
       coalesce(request_id,''), payload_json, attempts
from outbox_events
where status='PENDING'
  and next_attempt_at <= now()
  and (locked_at is null or locked_at < now() - interval '5 minutes')
order by next_attempt_at asc, occurred_at asc
for update skip locked
limit $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.OccurredAt, &e.AggregateType, &e.AggregateID, &e.Version, &e.RequestID, &e.PayloadJSON, &e.Attempts); err != nil {
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
    last_error=$4,
    locked_at=null,
    locked_by=null,
    updated_at=$3
where id=$1
`, id, nextAttemptAt, now, errMsg)
	return err
}

func (s *SQLStore) MarkTerminalFailed(ctx context.Context, db *sql.DB, id string, now time.Time, errMsg string) error {
	if db == nil {
		return fmt.Errorf("missing db")
	}
	if id == "" {
		return fmt.Errorf("missing id")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Snapshot row into dead letters.
	_, err = tx.ExecContext(ctx, `
insert into outbox_dead_letters (
  outbox_event_id, type, occurred_at,
  aggregate_type, aggregate_id, version,
  request_id, payload_json, attempts, last_error, failed_at
)
select id, type, occurred_at,
       aggregate_type, aggregate_id, version,
       request_id, payload_json, attempts+1, $2, $3
from outbox_events
where id=$1
`, id, errMsg, now)
	if err != nil {
		return err
	}

	// Mark terminal failed.
	_, err = tx.ExecContext(ctx, `
update outbox_events
set status='FAILED',
    attempts=attempts+1,
    last_error=$2,
    locked_at=null,
    locked_by=null,
    updated_at=$3
where id=$1
`, id, errMsg, now)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLStore) ReplayDeadLetters(ctx context.Context, db *sql.DB, limit int) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("missing db")
	}
	if limit <= 0 || limit > 10000 {
		limit = 100
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Pick newest dead letters first.
	rows, err := tx.QueryContext(ctx, `
select outbox_event_id
from outbox_dead_letters
order by failed_at desc
limit $1
for update skip locked
`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		return 0, nil
	}

	// Re-queue events: reset status/attempts/next_attempt.
	res, err := tx.ExecContext(ctx, `
update outbox_events
set status='PENDING',
    attempts=0,
    next_attempt_at=now(),
    locked_at=null,
    locked_by=null,
    last_error=null,
    updated_at=now()
where id = any($1)
`, pqStringArray(ids))
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()

	// Keep dead letters for auditability; they can be pruned later.
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
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

