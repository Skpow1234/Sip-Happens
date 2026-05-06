package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/orders"
)

// Verifier verifies Stripe webhook signatures and returns the verified event payload.
// Implementations should enforce replay tolerance.
type Verifier interface {
	Verify(payload []byte, stripeSignatureHeader string) error
}

// EventStore provides idempotency for webhook event application.
type EventStore interface {
	HasProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, eventID string, processedAt time.Time) error
}

type WebhookHandler struct {
	Verifier   Verifier
	EventStore EventStore
	Orders     orders.Service
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if h.Verifier == nil {
		http.Error(w, "server misconfigured", http.StatusInternalServerError)
		return
	}
	if err := h.Verifier.Verify(raw, r.Header.Get("Stripe-Signature")); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var evt stripeEvent
	if err := json.Unmarshal(raw, &evt); err != nil || evt.ID == "" || evt.Type == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if h.EventStore != nil {
		seen, err := h.EventStore.HasProcessed(ctx, evt.ID)
		if err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		if seen {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if err := h.applyEvent(ctx, evt); err != nil {
		// Domain already applied is a success response for Stripe retry semantics.
		if errors.Is(err, domain.ErrAlreadyApplied) {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if h.EventStore != nil {
		_ = h.EventStore.MarkProcessed(ctx, evt.ID, time.Now())
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) applyEvent(ctx context.Context, evt stripeEvent) error {
	// We intentionally keep event parsing minimal here.
	// Concrete mapping can be expanded once Stripe client choice is finalized.

	switch evt.Type {
	case "checkout.session.completed":
		// Expect metadata.order_id to exist.
		orderID := domain.OrderID(evt.Data.Object.Metadata.OrderID)
		if orderID == "" {
			return domain.ErrInvalidInput
		}
		payment := domain.Payment{
			Provider:    domain.PaymentProviderStripe,
			ProviderRef: evt.Data.Object.ID,
			Status:      domain.PaymentStatusSucceeded,
			RawPayload:  evt.RawObject,
		}
		return h.Orders.MarkPaid(ctx, orderID, payment)
	default:
		// Ignore unsupported event types.
		return nil
	}
}

// Minimal Stripe event envelope for our use.
// Note: Stripe's real schema is richer; this is a pragmatic subset.
type stripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	// RawObject preserves the event body for storage. Populated in custom UnmarshalJSON.
	RawObject string `json:"-"`

	Data struct {
		Object stripeCheckoutSession `json:"object"`
	} `json:"data"`
}

func (e *stripeEvent) UnmarshalJSON(b []byte) error {
	type alias stripeEvent
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	*e = stripeEvent(a)
	e.RawObject = string(b)
	return nil
}

type stripeCheckoutSession struct {
	ID       string `json:"id"`
	Metadata struct {
		OrderID string `json:"order_id"`
	} `json:"metadata"`
}

