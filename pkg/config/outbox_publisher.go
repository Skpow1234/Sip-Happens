package config

import (
	"fmt"
	"os"
	"strings"

	"sip-happens/pkg/events"
)

// OutboxPublisherKind selects how outbox-dispatch delivers events.
type OutboxPublisherKind string

const (
	OutboxPublisherLog  OutboxPublisherKind = "log"
	OutboxPublisherNATS OutboxPublisherKind = "nats"
)

// OutboxPublisherConfig is loaded from OUTBOX_PUBLISHER and related env vars.
type OutboxPublisherConfig struct {
	Kind OutboxPublisherKind

	NATSURL         string
	NATSStream      string
	Namespace       string
	SubjectPrefix   string
}

// LoadOutboxPublisher reads OUTBOX_PUBLISHER (default log when unset).
// Valid values: log (or empty), nats.
func LoadOutboxPublisher() (OutboxPublisherConfig, error) {
	var c OutboxPublisherConfig
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("OUTBOX_PUBLISHER")))
	switch raw {
	case "", "log":
		c.Kind = OutboxPublisherLog
		return c, nil
	case "nats":
		c.Kind = OutboxPublisherNATS
	default:
		return c, fmt.Errorf("unknown OUTBOX_PUBLISHER=%q (expected log or nats)", os.Getenv("OUTBOX_PUBLISHER"))
	}

	c.NATSURL = strings.TrimSpace(os.Getenv("NATS_URL"))
	if c.NATSURL == "" {
		return c, fmt.Errorf("NATS_URL is required when OUTBOX_PUBLISHER=nats")
	}
	c.NATSStream = strings.TrimSpace(os.Getenv("NATS_STREAM"))
	if c.NATSStream == "" {
		return c, fmt.Errorf("NATS_STREAM is required when OUTBOX_PUBLISHER=nats")
	}
	c.Namespace = strings.TrimSpace(os.Getenv("NAMESPACE"))
	if c.Namespace == "" {
		c.Namespace = "dev"
	}
	c.SubjectPrefix = strings.TrimSpace(os.Getenv("NATS_SUBJECT_PREFIX"))
	if c.SubjectPrefix == "" {
		c.SubjectPrefix = events.EventsSubjectPrefix(c.Namespace)
	} else if err := events.ValidateEventsSubjectPrefixForNamespace(c.Namespace, c.SubjectPrefix); err != nil {
		return c, err
	}
	return c, nil
}
