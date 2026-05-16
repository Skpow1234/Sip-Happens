package email

import (
	"strings"
	"testing"
)

func TestRenderPaymentLink(t *testing.T) {
	text, html, err := renderPaymentLink(PaymentLinkData{
		CustomerName: "Ana",
		OrderID:      "ord-1",
		PaymentURL:   "https://checkout.stripe.com/pay/cs_test",
		TotalDisplay: "COP 25000",
		ComplianceData: ComplianceData{
			PrivacyPolicyURL:  "https://privacy.example",
			ConsumerRightsURL: "https://sic.example",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "ord-1") || !strings.Contains(text, "checkout.stripe.com") {
		t.Fatalf("unexpected text: %q", text)
	}
	if !strings.Contains(html, "ord-1") {
		t.Fatalf("unexpected html: %q", html)
	}
	if !strings.Contains(text, "sic.example") || !strings.Contains(text, "menores") {
		t.Fatalf("expected compliance footer in text: %q", text)
	}
}

func TestRenderReceipt(t *testing.T) {
	text, _, err := renderReceipt(ReceiptData{
		OrderID:      "ord-2",
		TotalDisplay: "COP 1000",
		ItemsSummary: "- Beer x1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "ord-2") {
		t.Fatalf("unexpected: %q", text)
	}
}
