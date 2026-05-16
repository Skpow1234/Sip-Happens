package consumers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type MessageHandler interface {
	ConsumerName() string
	Handle(ctx context.Context, m *nats.Msg) error
}

type JetStreamRuntime struct {
	JS nats.JetStreamContext

	Stream        string
	FilterSubject string // e.g. "sip_happens.dev.events.>"
	Durable       string // e.g. "core-api"

	Idempotency IdempotencyStore

	DLQSubject string // e.g. "sip_happens.dev.dlq"

	BatchSize int
	MaxWait   time.Duration
}

func (r *JetStreamRuntime) Run(ctx context.Context, h MessageHandler) error {
	if r == nil || r.JS == nil {
		return fmt.Errorf("missing jetstream")
	}
	if strings.TrimSpace(r.Stream) == "" {
		return fmt.Errorf("missing stream")
	}
	if strings.TrimSpace(r.FilterSubject) == "" {
		return fmt.Errorf("missing filter subject")
	}
	if strings.TrimSpace(r.Durable) == "" {
		return fmt.Errorf("missing durable")
	}
	if r.Idempotency == nil {
		return fmt.Errorf("missing idempotency store")
	}
	if h == nil || strings.TrimSpace(h.ConsumerName()) == "" {
		return fmt.Errorf("missing handler/consumer name")
	}

	batch := r.BatchSize
	if batch <= 0 {
		batch = 50
	}
	wait := r.MaxWait
	if wait <= 0 {
		wait = 2 * time.Second
	}

	sub, err := r.JS.PullSubscribe(
		r.FilterSubject,
		r.Durable,
		nats.BindStream(r.Stream),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}

	log.Printf("consumer starting (stream=%s, filter=%s, durable=%s, consumer=%s)", r.Stream, r.FilterSubject, r.Durable, h.ConsumerName())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msgs, err := sub.Fetch(batch, nats.MaxWait(wait))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			return err
		}

		for _, msg := range msgs {
			if err := r.processOne(ctx, h, msg); err != nil {
				return err
			}
		}
	}
}

func (r *JetStreamRuntime) processOne(ctx context.Context, h MessageHandler, msg *nats.Msg) error {
	messageID := messageIDFromNATS(msg)
	seen, err := r.Idempotency.HasProcessed(ctx, h.ConsumerName(), messageID)
	if err != nil {
		return err
	}
	if seen {
		_ = msg.Ack()
		return nil
	}

	if err := h.Handle(ctx, msg); err != nil {
		eventType := msg.Header.Get("SipHappens-Event-Type")
		HandlerFailuresTotal.WithLabelValues(r.Stream, h.ConsumerName(), eventType).Inc()
		if dlqErr := r.publishDLQ(ctx, h, msg, err); dlqErr != nil {
			return fmt.Errorf("handler failed: %v; dlq publish failed: %w", err, dlqErr)
		}
		_ = msg.Ack()
		_ = r.Idempotency.MarkProcessed(ctx, h.ConsumerName(), messageID, time.Now())
		return nil
	}

	_ = msg.Ack()
	return r.Idempotency.MarkProcessed(ctx, h.ConsumerName(), messageID, time.Now())
}

func (r *JetStreamRuntime) publishDLQ(ctx context.Context, h MessageHandler, msg *nats.Msg, cause error) error {
	if strings.TrimSpace(r.DLQSubject) == "" {
		return nil
	}
	out := &nats.Msg{
		Subject: r.DLQSubject,
		Data:    msg.Data,
		Header:  nats.Header{},
	}
	for k, v := range msg.Header {
		for _, vv := range v {
			out.Header.Add(k, vv)
		}
	}
	out.Header.Set("SipHappens-DLQ-Error", cause.Error())

	_, err := r.JS.PublishMsg(out, nats.Context(ctx))
	if err == nil {
		consumer := ""
		if h != nil {
			consumer = h.ConsumerName()
		}
		DLQPublishedTotal.WithLabelValues(r.Stream, consumer).Inc()
	}
	return err
}

func messageIDFromNATS(msg *nats.Msg) string {
	if msg == nil {
		return ""
	}
	if id := msg.Header.Get("SipHappens-Event-Id"); id != "" {
		return id
	}
	sum := sha256.Sum256(msg.Data)
	return hex.EncodeToString(sum[:])
}
