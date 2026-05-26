package invoicing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/obslog"
)

// HTTPProvider is a generic JSON-over-HTTPS adapter against any DIAN
// authorized PT (Proveedor Tecnológico). It is intentionally NOT specific
// to one vendor — the request and response JSON shapes are defined by us
// and an integrator either:
//
//   (a) configures their PT to accept this shape via a custom integration
//       contract (most PTs let you POST a JSON body), or
//   (b) deploys a thin shim (Cloud Run / Lambda / nginx mod_js) that
//       translates between our shape and the PT's native shape.
//
// Option (b) is the recommended path for prod — it keeps the PT-specific
// auth (mutual TLS, OAuth, etc.) and XML signing concerns out of core-api
// so we can ship a new provider without redeploying the API.
//
// Request shape (POST {URL}, Authorization: Bearer {APIKey}):
//
//	{
//	  "order_id": "ord-…",
//	  "currency": "COP",
//	  "total_minor_units": 50000,
//	  "buyer": {
//	    "legal_id": "1020304050",
//	    "full_name": "Juan Pérez",
//	    "city": "Bogotá",
//	    "address": "Carrera 10 #11-22"
//	  },
//	  "line_items": [
//	    { "sku": "sku-1", "name": "Cerveza X", "qty": 2,
//	      "unit_minor": 5000, "line_minor": 10000 },
//	    …
//	  ]
//	}
//
// Response shape (HTTP 200 / 201):
//
//	{
//	  "external_id": "PT-2026-000123",
//	  "status": "ISSUED",            # ISSUED | PENDING | FAILED
//	  "cufe": "f9a4c1…384hex",
//	  "resolution_number": "18764028493",
//	  "resolution_prefix": "SETT",
//	  "sequence_number": 12345,
//	  "xml_url": "https://pt.example.co/invoices/abc.xml",
//	  "pdf_url": "https://pt.example.co/invoices/abc.pdf",
//	  "issued_at": "2026-05-26T12:34:56Z"
//	}
//
// Anything other than 2xx is treated as a transient FAILED — the admin
// retry endpoint (POST /admin/orders/{id}/invoice) reissues the request.
// 4xx responses with a JSON `{"error":"…"}` body propagate the error
// message into the obslog `dian_invoice_failed` line and the persisted
// `payload_json` for postmortem.
type HTTPProvider struct {
	URL    string
	APIKey string
	Client *http.Client
}

// NewHTTPProviderFromEnv reads INVOICE_HTTP_URL / INVOICE_HTTP_API_KEY /
// INVOICE_HTTP_TIMEOUT_SECONDS. If URL or APIKey is unset, it logs a
// warning and falls back to StubProvider-equivalent behaviour at call
// time (Issue returns FAILED with a descriptive message).
func NewHTTPProviderFromEnv() Provider {
	url := strings.TrimSpace(os.Getenv("INVOICE_HTTP_URL"))
	key := strings.TrimSpace(os.Getenv("INVOICE_HTTP_API_KEY"))
	timeoutSec := 15
	if v := strings.TrimSpace(os.Getenv("INVOICE_HTTP_TIMEOUT_SECONDS")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			timeoutSec = parsed
		}
	}
	return &HTTPProvider{
		URL:    url,
		APIKey: key,
		Client: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

func (p *HTTPProvider) Name() string { return "http" }

func (p *HTTPProvider) Issue(ctx context.Context, req IssueRequest) (IssueResult, error) {
	if p.URL == "" || p.APIKey == "" {
		return IssueResult{
			Status:      domain.InvoiceStatusFailed,
			PayloadJSON: `{"error":"INVOICE_HTTP_URL or INVOICE_HTTP_API_KEY not set"}`,
		}, errors.New("invoicing.http: missing URL or API key")
	}

	body := httpProviderRequest{
		OrderID:         string(req.OrderID),
		Currency:        req.Currency,
		TotalMinorUnits: req.TotalMinor,
		Buyer: httpProviderBuyer{
			LegalID:  req.LegalID,
			FullName: req.FullName,
			City:     req.City,
			Address:  req.Address,
		},
	}
	for _, li := range req.LineItems {
		body.LineItems = append(body.LineItems, httpProviderLineItem(li))
	}

	reqBody, err := json.Marshal(body)
	if err != nil {
		return IssueResult{Status: domain.InvoiceStatusFailed}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(reqBody))
	if err != nil {
		return IssueResult{Status: domain.InvoiceStatusFailed}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Idempotency-Key", "invoice-"+string(req.OrderID))

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		obslog.Warn(ctx, "dian_http_provider_call_failed", map[string]any{
			"order_id": string(req.OrderID),
			"err":      err.Error(),
		})
		return IssueResult{
			Status:      domain.InvoiceStatusFailed,
			PayloadJSON: fmt.Sprintf(`{"error":%q}`, err.Error()),
		}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		obslog.Warn(ctx, "dian_http_provider_http_error", map[string]any{
			"order_id":    string(req.OrderID),
			"status_code": resp.StatusCode,
			"body":        string(respBody),
		})
		return IssueResult{
			Status:      domain.InvoiceStatusFailed,
			PayloadJSON: string(respBody),
		}, fmt.Errorf("invoicing.http: provider returned HTTP %d", resp.StatusCode)
	}

	var parsed httpProviderResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return IssueResult{
			Status:      domain.InvoiceStatusFailed,
			PayloadJSON: string(respBody),
		}, fmt.Errorf("invoicing.http: invalid response body: %w", err)
	}

	status := domain.InvoiceStatus(strings.ToUpper(strings.TrimSpace(parsed.Status)))
	switch status {
	case domain.InvoiceStatusIssued, domain.InvoiceStatusPending, domain.InvoiceStatusFailed:
		// ok
	default:
		// Unknown status from provider — treat as PENDING so the admin
		// can investigate via the audit log without polluting metrics
		// with a FAILED row.
		status = domain.InvoiceStatusPending
	}

	out := IssueResult{
		ExternalID:       strings.TrimSpace(parsed.ExternalID),
		Status:           status,
		PayloadJSON:      string(respBody),
		CUFE:             strings.TrimSpace(parsed.CUFE),
		ResolutionNumber: strings.TrimSpace(parsed.ResolutionNumber),
		ResolutionPrefix: strings.TrimSpace(parsed.ResolutionPrefix),
		SequenceNumber:   parsed.SequenceNumber,
		XMLURL:           strings.TrimSpace(parsed.XMLURL),
		PDFURL:           strings.TrimSpace(parsed.PDFURL),
	}
	if out.ExternalID == "" {
		out.ExternalID = "http-" + string(req.OrderID)
	}
	return out, nil
}

type httpProviderRequest struct {
	OrderID         string                 `json:"order_id"`
	Currency        string                 `json:"currency"`
	TotalMinorUnits int64                  `json:"total_minor_units"`
	Buyer           httpProviderBuyer      `json:"buyer"`
	LineItems       []httpProviderLineItem `json:"line_items"`
}

type httpProviderBuyer struct {
	LegalID  string `json:"legal_id"`
	FullName string `json:"full_name"`
	City     string `json:"city"`
	Address  string `json:"address"`
}

type httpProviderLineItem struct {
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Qty       int    `json:"qty"`
	UnitMinor int64  `json:"unit_minor"`
	LineMinor int64  `json:"line_minor"`
}

type httpProviderResponse struct {
	ExternalID       string `json:"external_id"`
	Status           string `json:"status"`
	CUFE             string `json:"cufe"`
	ResolutionNumber string `json:"resolution_number"`
	ResolutionPrefix string `json:"resolution_prefix"`
	SequenceNumber   int64  `json:"sequence_number"`
	XMLURL           string `json:"xml_url"`
	PDFURL           string `json:"pdf_url"`
	IssuedAt         string `json:"issued_at"`
}
