package stripe

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/orders"
)

// ManualReconcileResponse is returned by POST /admin/orders/{id}/reconcile-stripe.
type ManualReconcileResponse struct {
	Outcome      string `json:"outcome"`
	StripeStatus string `json:"stripe_status,omitempty"`
	Message      string `json:"message,omitempty"`
}

// ManualReconcilePendingOrder polls Stripe for the order's payment intent and marks PAID when succeeded.
func ManualReconcilePendingOrder(ctx context.Context, o orders.Service, stripeSvc Service, id domain.OrderID) (ManualReconcileResponse, error) {
	var zero ManualReconcileResponse
	if o == nil || stripeSvc == nil {
		return zero, errors.New("missing dependencies")
	}
	ord, _, err := o.GetOrder(ctx, id)
	if err != nil {
		return zero, err
	}
	if ord.Status != domain.OrderStatusPendingPayment {
		return ManualReconcileResponse{
			Outcome: "skipped_not_pending",
			Message: string(ord.Status),
		}, nil
	}
	if ord.StripePaymentIntentID == "" {
		return zero, domain.ErrInvalidInput
	}

	st, err := stripeSvc.PaymentIntentStatus(ctx, ord.StripePaymentIntentID)
	if err != nil {
		return zero, err
	}

	now := time.Now()
	switch st {
	case "succeeded":
		payment := domain.Payment{
			Provider:    domain.PaymentProviderStripe,
			ProviderRef: ord.StripePaymentIntentID,
			Status:      domain.PaymentStatusSucceeded,
			RawPayload: fmt.Sprintf(`{"reconciled_at":"%s","stripe_payment_intent_id":"%s","stripe_status":"%s"}`,
				now.UTC().Format(time.RFC3339Nano), ord.StripePaymentIntentID, st),
		}
		if err := o.MarkPaid(ctx, id, payment); err != nil {
			if err == domain.ErrAlreadyApplied {
				return ManualReconcileResponse{Outcome: "already_paid", StripeStatus: st}, nil
			}
			return zero, err
		}
		return ManualReconcileResponse{Outcome: "marked_paid", StripeStatus: st}, nil
	case "processing", "requires_action", "requires_payment_method", "requires_confirmation":
		return ManualReconcileResponse{Outcome: "still_pending", StripeStatus: st}, nil
	default:
		return ManualReconcileResponse{
			Outcome:      "unhandled_stripe_status",
			StripeStatus: st,
			Message:      "requires manual review",
		}, nil
	}
}
