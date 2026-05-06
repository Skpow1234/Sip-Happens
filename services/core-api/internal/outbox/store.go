package outbox

import (
	"context"
	"database/sql"
	"time"
)

type Event struct {
	ID string

	Type       string
	OccurredAt time.Time

	AggregateType string
	AggregateID   string
	Version       int

	RequestID string
	PayloadJSON string
}

type Store interface {
	// Insert writes an outbox event inside the caller transaction.
	Insert(ctx context.Context, tx *sql.Tx, e Event) error

	// ClaimPending claims up to limit events ready to send and locks them.
	ClaimPending(ctx context.Context, db *sql.DB, now time.Time, limit int, lockedBy string) ([]Event, error)
	MarkSent(ctx context.Context, db *sql.DB, id string, sentAt time.Time) error
	MarkFailed(ctx context.Context, db *sql.DB, id string, now time.Time, errMsg string, nextAttemptAt time.Time) error
}

