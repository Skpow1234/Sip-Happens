package outbox

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math"
	"time"

	"sip-happens/pkg/obslog"
)

type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// LogPublisher is a temporary publisher used until we choose a real queue backend.
// It logs events in a structured-ish way and returns success.
type LogPublisher struct{}

func (p LogPublisher) Publish(ctx context.Context, e Event) error {
	ctx = obslog.WithRequestID(ctx, e.RequestID)
	if e.AggregateType == "order" {
		ctx = obslog.WithOrderID(ctx, e.AggregateID)
	}
	obslog.Info(ctx, "outbox_publish", map[string]any{
		"event_type":     e.Type,
		"event_id":       e.ID,
		"aggregate_type": e.AggregateType,
		"aggregate_id":   e.AggregateID,
		"version":        e.Version,
	})
	return nil
}

type Dispatcher struct {
	Store     Store
	Publisher Publisher

	LockedBy string
	Batch    int

	MaxAttempts int
	BaseBackoff time.Duration
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
	maxAttempts := d.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	base := d.BaseBackoff
	if base <= 0 {
		base = 30 * time.Second
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
			// Retry with exponential backoff + jitter; stop at MaxAttempts.
			nextAttempt := e.Attempts + 1
			if nextAttempt >= maxAttempts {
				_ = d.Store.MarkTerminalFailed(ctx, db, e.ID, time.Now(), err.Error())
			} else {
				next := time.Now().Add(backoffWithJitter(base, nextAttempt))
				_ = d.Store.MarkFailed(ctx, db, e.ID, time.Now(), err.Error(), next)
			}
			continue
		}
		if err := d.Store.MarkSent(ctx, db, e.ID, time.Now()); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func backoffWithJitter(base time.Duration, attempt int) time.Duration {
	// attempt starts at 1
	if attempt <= 0 {
		attempt = 1
	}
	// Exponential growth capped to 30 minutes.
	mult := math.Pow(2, float64(attempt-1))
	d := time.Duration(float64(base) * mult)
	if d > 30*time.Minute {
		d = 30 * time.Minute
	}
	// Jitter in [0.5, 1.5)
	j := 0.5 + randFloat()
	return time.Duration(float64(d) * j)
}

func randFloat() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	// convert to [0,1)
	u := uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 | uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	return float64(u>>11) / (1 << 53)
}
