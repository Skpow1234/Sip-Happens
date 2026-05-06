package events

import (
	"encoding/json"
	"errors"
	"time"
)

// Envelope is the canonical event wrapper used by the outbox and future message transports.
// It matches ADR 0006.
type Envelope struct {
	ID string `json:"id"`

	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurred_at"`

	AggregateType string `json:"aggregate_type"`
	AggregateID   string `json:"aggregate_id"`
	Version       int    `json:"version"`

	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

func (e Envelope) Validate() error {
	if e.ID == "" || e.Type == "" || e.AggregateType == "" || e.AggregateID == "" {
		return errors.New("missing required envelope fields")
	}
	if e.Version <= 0 {
		return errors.New("invalid version")
	}
	if e.OccurredAt.IsZero() {
		return errors.New("missing occurred_at")
	}
	if len(e.Payload) == 0 {
		return errors.New("missing payload")
	}
	return nil
}

func JSON(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

