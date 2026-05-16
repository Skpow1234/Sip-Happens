package outbox

import (
	"testing"

	"sip-happens/pkg/config"
)

func TestPublisherFromEnv_Log(t *testing.T) {
	t.Setenv("OUTBOX_PUBLISHER", "log")
	t.Setenv("NATS_URL", "")

	pub, cleanup, err := PublisherFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, ok := pub.(LogPublisher); !ok {
		t.Fatalf("expected LogPublisher, got %T", pub)
	}
}

func TestPublisherFromEnv_InvalidKind(t *testing.T) {
	t.Setenv("OUTBOX_PUBLISHER", "kafka")
	_, _, err := PublisherFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(interface{ Error() string }); !ok {
		t.Fatal(err)
	}
	_, err2 := config.LoadOutboxPublisher()
	if err2 == nil {
		t.Fatal("config should reject kafka")
	}
}
