package invoicing

import (
	"context"
	"testing"

	"sip-happens/pkg/domain"
)

func TestStubProvider_Issue(t *testing.T) {
	p := StubProvider{}
	res, err := p.Issue(context.Background(), IssueRequest{
		OrderID:    domain.OrderID("ord-1"),
		Currency:   "COP",
		TotalMinor: 50000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ExternalID != "stub-ord-1" {
		t.Fatalf("external_id=%q", res.ExternalID)
	}
	if res.Status != domain.InvoiceStatusPending {
		t.Fatalf("status=%q", res.Status)
	}
}
