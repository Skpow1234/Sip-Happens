package invoicing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"sip-happens/pkg/domain"
)

// IssueRequest is the minimum buyer and totals payload for a DIAN invoice (v1).
type IssueRequest struct {
	OrderID    domain.OrderID
	Currency   string
	TotalMinor int64
	LegalID    string
	FullName   string
	City       string
	Address    string
	LineItems  []IssueLineItem
}

type IssueLineItem struct {
	SKU       string
	Name      string
	Qty       int
	UnitMinor int64
	LineMinor int64
}

// IssueResult is returned by an authorized invoicing provider (or stub).
type IssueResult struct {
	ExternalID  string
	Status      domain.InvoiceStatus
	PayloadJSON string
}

// Provider issues or simulates a DIAN electronic invoice.
type Provider interface {
	Name() string
	Issue(ctx context.Context, req IssueRequest) (IssueResult, error)
}

// NewProviderFromEnv selects INVOICE_PROVIDER (default stub).
func NewProviderFromEnv() Provider {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("INVOICE_PROVIDER"))) {
	case "stub", "":
		return StubProvider{}
	default:
		return StubProvider{}
	}
}

// StubProvider logs-only stub for dev; returns PENDING with a deterministic external id.
type StubProvider struct{}

func (StubProvider) Name() string { return "stub" }

func (StubProvider) Issue(ctx context.Context, req IssueRequest) (IssueResult, error) {
	_ = ctx
	payload, _ := json.Marshal(map[string]any{
		"mode":     "stub",
		"order_id": string(req.OrderID),
		"total":    req.TotalMinor,
		"currency": req.Currency,
	})
	return IssueResult{
		ExternalID:  fmt.Sprintf("stub-%s", req.OrderID),
		Status:      domain.InvoiceStatusPending,
		PayloadJSON: string(payload),
	}, nil
}
