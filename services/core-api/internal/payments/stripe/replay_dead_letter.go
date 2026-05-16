package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"sip-happens/pkg/domain"
)

// ReplayResult describes one dead-letter replay attempt.
type ReplayResult struct {
	ID      int64  `json:"id"`
	EventID string `json:"event_id"`
	Status  string `json:"status"` // replayed | skipped | failed
	Error   string `json:"error,omitempty"`
}

// ReplayDeadLetterByID re-applies a stored Stripe webhook payload.
func (h *WebhookHandler) ReplayDeadLetterByID(ctx context.Context, log *SQLEventLogStore, id int64) (ReplayResult, error) {
	if h == nil || log == nil || log.DB == nil {
		return ReplayResult{}, fmt.Errorf("stripe replay not configured")
	}
	row, err := log.GetDeadLetterByID(ctx, id)
	if err != nil {
		return ReplayResult{}, err
	}
	if row == nil {
		return ReplayResult{}, domain.ErrNotFound
	}
	res := ReplayResult{ID: row.ID, EventID: row.EventID}
	if err := h.replayRaw(ctx, row.EventID, row.RawPayload); err != nil {
		if errors.Is(err, domain.ErrAlreadyApplied) {
			res.Status = "skipped"
			_ = log.DeleteDeadLetter(ctx, id)
			return res, nil
		}
		res.Status = "failed"
		res.Error = err.Error()
		now := time.Now()
		if row.EventID != "" {
			_ = log.MarkFailed(ctx, row.EventID, now, err.Error())
		}
		return res, err
	}
	res.Status = "replayed"
	_ = log.DeleteDeadLetter(ctx, id)
	return res, nil
}

// ReplayDeadLetters replays up to limit newest dead letters (newest first).
func (h *WebhookHandler) ReplayDeadLetters(ctx context.Context, log *SQLEventLogStore, limit int) ([]ReplayResult, error) {
	rows, err := log.ListDeadLetters(ctx, limit, 0)
	if err != nil {
		return nil, err
	}
	out := make([]ReplayResult, 0, len(rows))
	for _, row := range rows {
		res, err := h.ReplayDeadLetterByID(ctx, log, row.ID)
		if err != nil && res.Status == "" {
			res = ReplayResult{ID: row.ID, EventID: row.EventID, Status: "failed", Error: err.Error()}
		}
		out = append(out, res)
	}
	return out, nil
}

func (h *WebhookHandler) replayRaw(ctx context.Context, eventID, raw string) error {
	if raw == "" {
		return fmt.Errorf("empty dead-letter payload")
	}
	var evt stripeEvent
	if err := json.Unmarshal([]byte(raw), &evt); err != nil {
		return fmt.Errorf("parse stripe event: %w", err)
	}
	if evt.ID == "" {
		evt.ID = eventID
	}
	if evt.ID == "" {
		return fmt.Errorf("missing stripe event id")
	}

	if h.EventStore != nil {
		seen, err := h.EventStore.HasProcessed(ctx, evt.ID)
		if err != nil {
			return err
		}
		if seen {
			return domain.ErrAlreadyApplied
		}
	}

	if err := h.applyEvent(ctx, evt); err != nil {
		if errors.Is(err, domain.ErrAlreadyApplied) {
			return err
		}
		return err
	}

	now := time.Now()
	if h.EventStore != nil {
		_ = h.EventStore.MarkProcessed(ctx, evt.ID, now)
	}
	if h.EventLog != nil {
		_ = h.EventLog.MarkProcessed(ctx, evt.ID, now)
	}
	return nil
}
