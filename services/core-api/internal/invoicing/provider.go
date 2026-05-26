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
// All DIAN-specific fields are optional — the stub leaves them empty and
// only sets ExternalID / Status / PayloadJSON. A real authorized provider
// SHOULD populate CUFE, ResolutionNumber/Prefix, SequenceNumber, and
// XMLURL/PDFURL so the columns added in migration 030 are usable.
type IssueResult struct {
	ExternalID       string
	Status           domain.InvoiceStatus
	PayloadJSON      string
	CUFE             string
	ResolutionNumber string
	ResolutionPrefix string
	SequenceNumber   int64
	XMLURL           string
	PDFURL           string
}

// Provider issues or simulates a DIAN electronic invoice.
type Provider interface {
	Name() string
	Issue(ctx context.Context, req IssueRequest) (IssueResult, error)
}

// NewProviderFromEnv selects INVOICE_PROVIDER and assembles a provider
// from the relevant env vars.
//
// Supported values:
//
//   - ""       / "stub"  – StubProvider (default, dev-only).
//   - "http"             – HTTPProvider, a generic JSON-over-HTTPS adapter.
//     Required env: INVOICE_HTTP_URL, INVOICE_HTTP_API_KEY,
//     INVOICE_HTTP_TIMEOUT_SECONDS (optional, default 15s).
//     This is intentionally generic so the same code talks to Factura1,
//     Alegra, Carvajal, Siigo, etc. — wire the actual endpoint and key
//     for your contracted provider via env. The expected JSON request/
//     response shape is documented on HTTPProvider.Issue.
//
// Unknown values fall through to StubProvider with a startup log line,
// so a typo in INVOICE_PROVIDER doesn't silently send real invoices to
// the stub.
func NewProviderFromEnv() Provider {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("INVOICE_PROVIDER"))) {
	case "stub", "":
		return StubProvider{}
	case "http":
		return NewHTTPProviderFromEnv()
	default:
		// Fall through to the stub so a mistyped env doesn't take down
		// the API. The caller (cmd/api/main.go) logs the active
		// provider via NewProviderFromEnv().Name().
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
