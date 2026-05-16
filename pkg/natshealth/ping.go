package natshealth

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Ping verifies NATS is reachable (connect + flush).
func Ping(ctx context.Context, natsURL string) error {
	if natsURL == "" {
		return fmt.Errorf("NATS_URL is empty")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("sip-happens-readyz"),
		nats.Timeout(2*time.Second),
		nats.MaxReconnects(0),
	)
	if err != nil {
		return err
	}
	defer nc.Close()

	done := make(chan error, 1)
	go func() { done <- nc.Flush() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
