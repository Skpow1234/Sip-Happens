package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// LogPublisher is a temporary publisher used until we choose a real queue backend.
// It logs events in a structured-ish way and returns success.
type LogPublisher struct{}

func (p LogPublisher) Publish(ctx context.Context, e Event) error {
	_ = ctx
	log.Printf("outbox publish type=%s id=%s aggregate=%s/%s v=%d request_id=%s", e.Type, e.ID, e.AggregateType, e.AggregateID, e.Version, e.RequestID)
	return nil
}

type Dispatcher struct {
	Store     Store
	Publisher Publisher

	LockedBy string
	Batch    int
}

func (d *Dispatcher) RunOnce(ctx context.Context, db *sql.DB, now time.Time) (int, error) {
	if d.Store == nil || d.Publisher == nil {
		return 0, fmt.Errorf("missing store/publisher")
	}
	if db == nil {
		return 0, fmt.Errorf("missing db")
	}
	limit := d.Batch
	if limit <= 0 {
		limit = 100
	}
	events, err := d.Store.ClaimPending(ctx, db, now, limit, d.LockedBy)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}

	sent := 0
	for _, e := range events {
		if err := d.Publisher.Publish(ctx, e); err != nil {
			// retry with backoff based on attempts (attempt count increments in DB).
			_ = d.Store.MarkFailed(ctx, db, e.ID, time.Now(), err.Error(), time.Now().Add(30*time.Second))
			continue
		}
		if err := d.Store.MarkSent(ctx, db, e.ID, time.Now()); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

