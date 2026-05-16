package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"sip-happens/pkg/ids"
)

// InsertPendingEvents inserts n PENDING outbox rows (for integration / multi-node smoke tests).
func InsertPendingEvents(ctx context.Context, db *sql.DB, eventType string, n int) error {
	if db == nil {
		return fmt.Errorf("missing db")
	}
	if eventType == "" {
		return fmt.Errorf("missing eventType")
	}
	if n <= 0 {
		return fmt.Errorf("n must be > 0")
	}

	store := NewSQLStore()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	for i := 0; i < n; i++ {
		id, err := ids.UUIDv4()
		if err != nil {
			return err
		}
		agg, err := ids.UUIDv4()
		if err != nil {
			return err
		}
		e := Event{
			ID:            id,
			Type:          eventType,
			OccurredAt:    now,
			AggregateType: "test",
			AggregateID:   agg,
			Version:       1,
			PayloadJSON:   `{"ok":true}`,
		}
		if err := store.Insert(ctx, tx, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CountOutboxByStatus returns how many outbox_events rows match type and status.
func CountOutboxByStatus(ctx context.Context, db *sql.DB, eventType, status string) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("missing db")
	}
	var n int
	err := db.QueryRowContext(ctx, `
select count(*) from outbox_events where type=$1 and status=$2
`, eventType, status).Scan(&n)
	return n, err
}
