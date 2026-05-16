package audit

import (
	"context"
	"database/sql"
	"encoding/json"

	"sip-happens/pkg/redact"
)

// Querier executes SQL ( *sql.DB or *sql.Tx ).
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Store appends rows to audit_events.
type Store struct{}

func NewSQLStore() *Store { return &Store{} }

// Insert writes an audit_events row. payloadJSON is redacted before persist.
func (Store) Insert(ctx context.Context, q Querier, aggregateType, aggregateID, actor, eventType, payloadJSON string) error {
	if q == nil {
		return nil
	}
	if actor == "" {
		actor = "system"
	}
	payloadJSON = redact.JSON(payloadJSON)
	_, err := q.ExecContext(ctx, `
insert into audit_events (aggregate_type, aggregate_id, actor, event_type, payload_json, created_at)
values ($1,$2,$3,$4,$5,now())
`, aggregateType, aggregateID, actor, eventType, payloadJSON)
	return err
}

// InsertMap JSON-encodes payload and calls Insert.
func (s Store) InsertMap(ctx context.Context, q Querier, aggregateType, aggregateID, actor, eventType string, payload map[string]any) error {
	if len(payload) == 0 {
		return s.Insert(ctx, q, aggregateType, aggregateID, actor, eventType, "{}")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Insert(ctx, q, aggregateType, aggregateID, actor, eventType, string(b))
}
