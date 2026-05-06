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
}

