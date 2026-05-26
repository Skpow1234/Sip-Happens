package consumers

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// IdempotencyStore provides dedupe per consumer by message id.
type IdempotencyStore interface {
	HasProcessed(ctx context.Context, consumerName, messageID string) (bool, error)
	MarkProcessed(ctx context.Context, consumerName, messageID string, processedAt time.Time) error
}

type SQLIdempotencyStore struct {
	DB *sql.DB
}

func (s *SQLIdempotencyStore) HasProcessed(ctx context.Context, consumerName, messageID string) (bool, error) {
	if s.DB == nil {
		return false, fmt.Errorf("missing db")
	}
	row := s.DB.QueryRowContext(ctx, `
select 1
from consumer_processed_messages
where consumer_name=$1 and message_id=$2
`, consumerName, messageID)
	var one int
	if err := row.Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *SQLIdempotencyStore) MarkProcessed(ctx context.Context, consumerName, messageID string, processedAt time.Time) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if consumerName == "" || messageID == "" {
		return fmt.Errorf("missing consumerName/messageID")
	}
	if processedAt.IsZero() {
		processedAt = time.Now()
	}
	_, err := s.DB.ExecContext(ctx, `
insert into consumer_processed_messages (consumer_name, message_id, processed_at)
values ($1,$2,$3)
on conflict (consumer_name, message_id) do nothing
`, consumerName, messageID, processedAt)
	return err
}
