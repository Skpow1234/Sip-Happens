package outbox

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"sip-happens/pkg/events"
)

const (
	NatsHeaderEventID          = "SipHappens-Event-Id"
	NatsHeaderEventType        = "SipHappens-Event-Type"
	NatsHeaderAggregateType    = "SipHappens-Aggregate-Type"
	NatsHeaderAggregateID      = "SipHappens-Aggregate-Id"
	NatsHeaderEventVersion     = "SipHappens-Event-Version"
	NatsHeaderOccurredAt       = "SipHappens-Occurred-At"
	NatsHeaderRequestID        = "SipHappens-Request-Id"
	NatsHeaderContentType      = "Content-Type"
	NatsContentTypeJSON        = "application/json"
	defaultPublishTimeout      = 5 * time.Second
	defaultEnsureStreamTimeout = 5 * time.Second
)

type JetStreamPublisher struct {
	JS nats.JetStreamContext

	Stream        string
	SubjectPrefix string // e.g. "sip_happens.dev.events"

	PublishTimeout time.Duration

	once    sync.Once
	initErr error
}

func (p *JetStreamPublisher) Publish(ctx context.Context, e Event) error {
	if p == nil || p.JS == nil {
		return fmt.Errorf("nats publisher not configured")
	}
	if strings.TrimSpace(p.Stream) == "" {
		return fmt.Errorf("nats stream is required")
	}
	if strings.TrimSpace(p.SubjectPrefix) == "" {
		return fmt.Errorf("nats subject prefix is required")
	}

	p.once.Do(func() {
		initCtx, cancel := context.WithTimeout(context.Background(), ensureStreamTimeout(p.PublishTimeout))
		defer cancel()
		p.initErr = p.ensureStream(initCtx)
	})
	if p.initErr != nil {
		return p.initErr
	}

	if err := events.ValidateEventType(e.Type); err != nil {
		return err
	}
	subject, err := p.subjectForEvent(e)
	if err != nil {
		return err
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    []byte(e.PayloadJSON),
		Header:  nats.Header{},
	}
	msg.Header.Set(NatsHeaderContentType, NatsContentTypeJSON)
	msg.Header.Set(NatsHeaderEventID, e.ID)
	msg.Header.Set(NatsHeaderEventType, e.Type)
	msg.Header.Set(NatsHeaderAggregateType, e.AggregateType)
	msg.Header.Set(NatsHeaderAggregateID, e.AggregateID)
	msg.Header.Set(NatsHeaderEventVersion, fmt.Sprintf("%d", e.Version))
	msg.Header.Set(NatsHeaderOccurredAt, e.OccurredAt.UTC().Format(time.RFC3339Nano))
	if e.RequestID != "" {
		msg.Header.Set(NatsHeaderRequestID, e.RequestID)
	}

	timeout := p.PublishTimeout
	if timeout <= 0 {
		timeout = defaultPublishTimeout
	}
	pubCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, pubErr := p.JS.PublishMsg(msg, nats.Context(pubCtx))
	return pubErr
}

func (p *JetStreamPublisher) ensureStream(ctx context.Context) error {
	_, err := p.JS.StreamInfo(p.Stream, nats.Context(ctx))
	if err == nil {
		return nil
	}
	if err != nats.ErrStreamNotFound {
		return err
	}

	cfg := &nats.StreamConfig{
		Name:     p.Stream,
		Subjects: []string{strings.TrimSuffix(p.SubjectPrefix, ".") + ".>"},
		Storage:  nats.FileStorage,
		// Conservative defaults; consumers decide their own ack/retry behavior.
		Retention: nats.LimitsPolicy,
	}
	_, err = p.JS.AddStream(cfg, nats.Context(ctx))
	return err
}

func (p *JetStreamPublisher) subjectForEvent(e Event) (string, error) {
	prefix := strings.TrimSuffix(strings.TrimSpace(p.SubjectPrefix), ".")
	if err := events.ValidateEventsSubjectPrefixFormat(prefix); err != nil {
		return "", err
	}
	return prefix + "." + e.Type, nil
}

func ensureStreamTimeout(publishTimeout time.Duration) time.Duration {
	if publishTimeout <= 0 {
		return defaultEnsureStreamTimeout
	}
	if publishTimeout < time.Second {
		return time.Second
	}
	return publishTimeout
}
