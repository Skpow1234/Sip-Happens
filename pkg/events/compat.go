package events

import (
	"encoding/json"
	"fmt"
)

// Decoder helps consumers enforce versioned payload decoding rules.
// It is intentionally tiny: it does not perform IO, queue operations, or retries.
type Decoder struct {
	// KnownVersions maps event type -> supported versions.
	// Example: {"orders.order.paid": {1: struct{}{}}}
	KnownVersions map[string]map[int]struct{}
}

// DecodePayload decodes an envelope payload into out if the type/version is supported.
// Consumers should treat unknown versions as non-retryable until code is updated.
func (d Decoder) DecodePayload(e Envelope, out any) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if !d.supports(e.Type, e.Version) {
		return fmt.Errorf("unsupported event version: type=%s version=%d", e.Type, e.Version)
	}
	if err := json.Unmarshal(e.Payload, out); err != nil {
		return fmt.Errorf("invalid payload json: %w", err)
	}
	return nil
}

func (d Decoder) supports(typ string, version int) bool {
	if d.KnownVersions == nil {
		return false
	}
	vs := d.KnownVersions[typ]
	if vs == nil {
		return false
	}
	_, ok := vs[version]
	return ok
}
