package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/obslog"
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

// RefundReconciler is the minimal surface the webhook handler needs from
// the refunds service. Declared as a local interface to avoid a circular
// import (refunds depends on stripe.Service).
type RefundReconciler interface {
	MarkRefundStatusByStripeID(ctx context.Context, stripeRefundID, newStatus, failureMessage string) error
}

type WebhookHandler struct {
	Verifier   Verifier
	EventStore EventStore
	EventLog   EventLogStore
	Orders     orders.Service
	OrderRepo  orders.Repository
	OrderQuery orders.Querier
	// Refunds is the refund reconciler invoked on `charge.refunded` /
	// `refund.updated` events. Optional; when nil the events still
	// short-circuit to MarkIgnored without erroring.
	Refunds RefundReconciler
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

	orderIDFromPayload := evt.ExtractOrderID()
	if h.EventLog != nil {
		_ = h.EventLog.UpsertReceived(ctx, evt.ID, evt.Type, orderIDFromPayload, evt.RawObject, time.Now())
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
			if h.EventLog != nil {
				_ = h.EventLog.MarkProcessed(ctx, evt.ID, time.Now())
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if h.EventLog != nil {
			_ = h.EventLog.MarkFailed(ctx, evt.ID, time.Now(), err.Error())
			_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "apply_failed", evt.RawObject)
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if h.EventStore != nil {
		_ = h.EventStore.MarkProcessed(ctx, evt.ID, time.Now())
	}
	if h.EventLog != nil {
		_ = h.EventLog.MarkProcessed(ctx, evt.ID, time.Now())
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) resolveOrderIDFromCheckoutSession(ctx context.Context, evt stripeEvent, obj stripeCheckoutSession) (domain.OrderID, error) {
	orderID := domain.OrderID(obj.Metadata.OrderID)
	if orderID == "" && h.OrderRepo != nil && obj.ID != "" {
		if found, err := h.OrderRepo.FindByStripeCheckoutSessionID(ctx, h.OrderQuery, obj.ID); err == nil {
			orderID = found
		}
	}
	if orderID == "" {
		if h.EventLog != nil {
			_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "missing_order_id", evt.RawObject)
		}
		return "", domain.ErrInvalidInput
	}
	return orderID, nil
}

func (h *WebhookHandler) applyEvent(ctx context.Context, evt stripeEvent) error {
	switch evt.Type {
	case "checkout.session.completed":
		obj, err := evt.AsCheckoutSession()
		if err != nil {
			if h.EventLog != nil {
				_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "invalid_checkout_session_payload", evt.RawObject)
			}
			return domain.ErrInvalidInput
		}

		orderID, err := h.resolveOrderIDFromCheckoutSession(ctx, evt, obj)
		if err != nil {
			return err
		}
		payment := domain.Payment{
			Provider:    domain.PaymentProviderStripe,
			ProviderRef: obj.ID,
			Status:      domain.PaymentStatusSucceeded,
			RawPayload:  evt.RawObject,
		}
		if err := h.Orders.MarkPaid(ctx, orderID, payment); err != nil {
			return err
		}
		obslog.Info(obslog.WithOrderID(ctx, string(orderID)), "stripe_webhook_applied", map[string]any{
			"stripe_event_id": evt.ID,
			"stripe_type":     evt.Type,
		})
		return nil
	case "checkout.session.expired":
		obj, err := evt.AsCheckoutSession()
		if err != nil {
			if h.EventLog != nil {
				_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "invalid_checkout_session_payload", evt.RawObject)
			}
			return domain.ErrInvalidInput
		}

		orderID, err := h.resolveOrderIDFromCheckoutSession(ctx, evt, obj)
		if err != nil {
			return err
		}
		err = h.Orders.CancelPendingOrder(ctx, orderID, "stripe_session_expired")
		if errors.Is(err, domain.ErrConflict) {
			// Order already paid or otherwise not cancellable; treat as handled.
			return nil
		}
		return err
	case "payment_intent.succeeded":
		obj, err := evt.AsPaymentIntent()
		if err != nil {
			if h.EventLog != nil {
				_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "invalid_payment_intent_payload", evt.RawObject)
			}
			return domain.ErrInvalidInput
		}

		orderID := domain.OrderID(obj.Metadata.OrderID)
		if orderID == "" {
			// Fallback to DB lookup by Stripe payment intent id.
			if h.OrderRepo != nil && obj.ID != "" {
				found, err := h.OrderRepo.FindByStripePaymentIntentID(ctx, h.OrderQuery, obj.ID)
				if err == nil {
					orderID = found
				}
			}
		}
		if orderID == "" {
			if h.EventLog != nil {
				_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "missing_order_id", evt.RawObject)
			}
			return domain.ErrInvalidInput
		}

		payment := domain.Payment{
			Provider:    domain.PaymentProviderStripe,
			ProviderRef: obj.ID,
			Status:      domain.PaymentStatusSucceeded,
			RawPayload:  evt.RawObject,
		}
		return h.Orders.MarkPaid(ctx, orderID, payment)
	case "charge.refunded", "refund.updated", "refund.created":
		// Stripe sends `charge.refunded` when a refund is initiated and
		// `refund.updated` when its status changes. The payload shapes
		// differ: `charge.*` has a `refunds.data[]` array of refund
		// objects; `refund.*` has the refund object directly. We try
		// both shapes and forward each refund to the reconciler.
		if h.Refunds == nil {
			if h.EventLog != nil {
				_ = h.EventLog.MarkIgnored(ctx, evt.ID, time.Now())
			}
			return nil
		}
		refundObjs, err := evt.AsRefundObjects()
		if err != nil {
			if h.EventLog != nil {
				_ = h.EventLog.InsertDeadLetter(ctx, evt.ID, "invalid_refund_payload", evt.RawObject)
			}
			return domain.ErrInvalidInput
		}
		var firstErr error
		for _, r := range refundObjs {
			if err := h.Refunds.MarkRefundStatusByStripeID(ctx, r.ID, r.Status, r.FailureReason); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	default:
		// Ignore unsupported event types.
		if h.EventLog != nil {
			_ = h.EventLog.MarkIgnored(ctx, evt.ID, time.Now())
		}
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
		Object json.RawMessage `json:"object"`
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

func (e stripeEvent) ExtractOrderID() string {
	// Best-effort extraction for logging/indexing purposes.
	switch e.Type {
	case "checkout.session.completed", "checkout.session.expired":
		if cs, err := e.AsCheckoutSession(); err == nil {
			return cs.Metadata.OrderID
		}
	case "payment_intent.succeeded":
		if pi, err := e.AsPaymentIntent(); err == nil {
			return pi.Metadata.OrderID
		}
	}
	return ""
}

func (e stripeEvent) AsCheckoutSession() (stripeCheckoutSession, error) {
	var cs stripeCheckoutSession
	if len(e.Data.Object) == 0 {
		return cs, errors.New("missing object")
	}
	if err := json.Unmarshal(e.Data.Object, &cs); err != nil {
		return cs, err
	}
	if cs.ID == "" {
		return cs, errors.New("missing id")
	}
	return cs, nil
}

func (e stripeEvent) AsPaymentIntent() (stripePaymentIntent, error) {
	var pi stripePaymentIntent
	if len(e.Data.Object) == 0 {
		return pi, errors.New("missing object")
	}
	if err := json.Unmarshal(e.Data.Object, &pi); err != nil {
		return pi, err
	}
	if pi.ID == "" {
		return pi, errors.New("missing id")
	}
	return pi, nil
}

type stripeCheckoutSession struct {
	ID       string `json:"id"`
	Metadata struct {
		OrderID string `json:"order_id"`
	} `json:"metadata"`
}

type stripePaymentIntent struct {
	ID       string `json:"id"`
	Metadata struct {
		OrderID string `json:"order_id"`
	} `json:"metadata"`
}
