package config

import (
	"testing"
)

func TestLoadOutboxPublisher_DefaultLog(t *testing.T) {
	t.Setenv("OUTBOX_PUBLISHER", "")
	t.Setenv("NATS_URL", "")
	cfg, err := LoadOutboxPublisher()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != OutboxPublisherLog {
		t.Fatalf("kind=%q", cfg.Kind)
	}
}

func TestLoadOutboxPublisher_NATSRequiresURL(t *testing.T) {
	t.Setenv("OUTBOX_PUBLISHER", "nats")
	t.Setenv("NATS_URL", "")
	t.Setenv("NATS_STREAM", "sip_happens")
	_, err := LoadOutboxPublisher()
	if err == nil {
		t.Fatal("expected error when NATS_URL missing")
	}
}

func TestLoadOutboxPublisher_NATSOK(t *testing.T) {
	t.Setenv("OUTBOX_PUBLISHER", "nats")
	t.Setenv("NATS_URL", "nats://localhost:4222")
	t.Setenv("NATS_STREAM", "sip_happens")
	t.Setenv("NAMESPACE", "staging")
	cfg, err := LoadOutboxPublisher()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != OutboxPublisherNATS || cfg.NATSURL == "" || cfg.NATSStream == "" {
		t.Fatalf("cfg=%+v", cfg)
	}
}
