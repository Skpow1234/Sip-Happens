package domain

import "time"

type AuditEvent struct {
	AggregateType string
	AggregateID   string
	Actor         string
	EventType     string
	PayloadJSON   string
	CreatedAt     time.Time
}

