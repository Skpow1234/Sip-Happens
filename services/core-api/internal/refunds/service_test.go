package refunds

import (
	"strings"
	"testing"

	"sip-happens/pkg/domain"
)

// TestIdempotencyKey_StableAndDistinct guards the two contracts we rely
// on for Stripe-side dedupe:
//
//  1. Retrying the *same* logical refund (same order + amount + reason)
//     must produce the exact same key, so Stripe collapses retries.
//  2. Two genuinely different refunds against the same order (different
//     amount or different reason) must produce different keys, so Stripe
//     correctly issues both.
//
// If either property breaks, partial refunds either silently no-op or
// double-charge customers — both money-losing bugs.
func TestIdempotencyKey_StableAndDistinct(t *testing.T) {
	const order = domain.OrderID("ord_test_1")

	k1 := IdempotencyKey(order, 1500, "wrong_item")
	k2 := IdempotencyKey(order, 1500, "wrong_item")
	if k1 != k2 {
		t.Fatalf("same (order, amount, reason) must produce equal keys: %q vs %q", k1, k2)
	}
	if !strings.HasPrefix(k1, "refund-") {
		t.Fatalf("key should be prefixed with refund-, got %q", k1)
	}

	cases := []struct {
		name   string
		order  domain.OrderID
		amount int64
		reason string
	}{
		{"different_amount", order, 2000, "wrong_item"},
		{"different_reason", order, 1500, "shipping_issue"},
		{"different_order", domain.OrderID("ord_test_2"), 1500, "wrong_item"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := IdempotencyKey(tc.order, tc.amount, tc.reason)
			if k == k1 {
				t.Fatalf("%s should produce a distinct key, got the same %q", tc.name, k)
			}
		})
	}
}

// TestIdempotencyKey_ReasonWhitespaceNormalised ensures retries from a
// caller that re-trimmed the reason don't get a different key.
func TestIdempotencyKey_ReasonWhitespaceNormalised(t *testing.T) {
	const order = domain.OrderID("ord_test_1")
	k1 := IdempotencyKey(order, 1500, "wrong_item")
	k2 := IdempotencyKey(order, 1500, "  wrong_item  ")
	if k1 != k2 {
		t.Fatalf("leading/trailing whitespace on reason should not change key: %q vs %q", k1, k2)
	}
}

func TestNormalizeStripeRefundStatus(t *testing.T) {
	cases := map[string]domain.RefundStatus{
		"succeeded":       domain.RefundStatusSucceeded,
		"SUCCEEDED":       domain.RefundStatusSucceeded,
		"  succeeded  ":   domain.RefundStatusSucceeded,
		"failed":          domain.RefundStatusFailed,
		"canceled":        domain.RefundStatusFailed,
		"cancelled":       domain.RefundStatusFailed,
		"pending":         domain.RefundStatusPending,
		"requires_action": domain.RefundStatusPending,
		"":                domain.RefundStatusPending,
		"unknown_garbage": domain.RefundStatusPending,
	}
	for in, want := range cases {
		if got := normalizeStripeRefundStatus(in); got != want {
			t.Errorf("normalizeStripeRefundStatus(%q): got %q want %q", in, got, want)
		}
	}
}

func TestIsRefundableStatus(t *testing.T) {
	refundable := []domain.OrderStatus{
		domain.OrderStatusPaid,
		domain.OrderStatusFulfilling,
		domain.OrderStatusShipped,
		domain.OrderStatusRefunded, // partial refund top-up
	}
	for _, s := range refundable {
		if !isRefundableStatus(s) {
			t.Errorf("status %q should be refundable", s)
		}
	}
	notRefundable := []domain.OrderStatus{
		domain.OrderStatusPendingPayment,
		domain.OrderStatusCancelled,
		domain.OrderStatus("DRAFT"),
		domain.OrderStatus(""),
	}
	for _, s := range notRefundable {
		if isRefundableStatus(s) {
			t.Errorf("status %q should NOT be refundable", s)
		}
	}
}
