package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"

	"sip-happens/pkg/config"
)

// PublisherFromEnv builds the outbox publisher selected by OUTBOX_PUBLISHER.
// cleanup must be called when non-nil (closes NATS connection for nats publisher).
func PublisherFromEnv() (Publisher, func(), error) {
	cfg, err := config.LoadOutboxPublisher()
	if err != nil {
		return nil, nil, err
	}
	switch cfg.Kind {
	case config.OutboxPublisherLog:
		return LogPublisher{}, func() {}, nil
	case config.OutboxPublisherNATS:
		return publisherFromNATS(cfg)
	default:
		return nil, nil, fmt.Errorf("unsupported outbox publisher kind %q", cfg.Kind)
	}
}

func publisherFromNATS(cfg config.OutboxPublisherConfig) (Publisher, func(), error) {
	nc, err := nats.Connect(cfg.NATSURL, nats.Name("sip-happens outbox-dispatch"))
	if err != nil {
		return nil, nil, err
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, err
	}
	pub := &JetStreamPublisher{
		JS:            js,
		Stream:        cfg.NATSStream,
		SubjectPrefix: cfg.SubjectPrefix,
	}
	ensureCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pub.ensureStream(ensureCtx); err != nil {
		nc.Close()
		return nil, nil, err
	}
	return pub, func() { nc.Close() }, nil
}
