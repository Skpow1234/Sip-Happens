package stripe

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestNormalizePaymentIntentStatus(t *testing.T) {
	if got := NormalizePaymentIntentStatus("succeeded"); got != domain.PaymentStatusSucceeded {
		t.Fatalf("got %q", got)
	}
	if got := NormalizePaymentIntentStatus("processing"); got != domain.PaymentStatusPending {
		t.Fatalf("got %q", got)
	}
	if got := NormalizePaymentIntentStatus("canceled"); got != domain.PaymentStatusFailed {
		t.Fatalf("got %q", got)
	}
}
