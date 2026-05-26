package invoicing

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/audit"
	"sip-happens/services/core-api/internal/orders"
)

// Service persists invoice rows and calls the configured Provider.
type Service struct {
	DB       *sql.DB
	Provider Provider
	Orders   orders.Service
	Audit    *audit.Store
}

func (s *Service) GetByOrderID(ctx context.Context, orderID domain.OrderID) (*domain.Invoice, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	if orderID == "" {
		return nil, domain.ErrInvalidInput
	}
	row := s.DB.QueryRowContext(ctx, `
select id, order_id, provider, coalesce(external_id,''), status,
       coalesce(payload_json::text,''),
       coalesce(cufe,''), coalesce(resolution_number,''),
       coalesce(resolution_prefix,''), coalesce(sequence_number, 0),
       coalesce(xml_url,''), coalesce(pdf_url,''), issued_at,
       created_at, updated_at
from invoices where order_id=$1
`, string(orderID))
	return scanInvoice(row)
}

// IssueForOrder creates or retries an invoice for a paid order (idempotent when ISSUED).
func (s *Service) IssueForOrder(ctx context.Context, orderID domain.OrderID, actor string) (*domain.Invoice, error) {
	if s == nil || s.DB == nil || s.Provider == nil || s.Orders == nil {
		return nil, fmt.Errorf("invoicing service misconfigured")
	}
	if orderID == "" {
		return nil, domain.ErrInvalidInput
	}

	o, items, err := s.Orders.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if o.Status != domain.OrderStatusPaid &&
		o.Status != domain.OrderStatusFulfilling &&
		o.Status != domain.OrderStatusShipped {
		return nil, domain.ErrInvalidInput
	}

	existing, err := s.GetByOrderID(ctx, orderID)
	if err != nil && err != domain.ErrNotFound {
		return nil, err
	}
	if existing != nil && existing.Status == domain.InvoiceStatusIssued {
		return existing, nil
	}

	invID := ""
	if existing != nil {
		invID = existing.ID
	} else {
		u, err := ids.UUIDv4()
		if err != nil {
			return nil, err
		}
		invID = u
		now := time.Now().UTC()
		_, err = s.DB.ExecContext(ctx, `
insert into invoices (id, order_id, provider, external_id, status, payload_json, created_at, updated_at)
values ($1,$2,$3,'', $4, '{}'::jsonb, $5, $5)
`, invID, string(orderID), s.Provider.Name(), string(domain.InvoiceStatusPending), now)
		if err != nil {
			return nil, err
		}
	}

	req := buildIssueRequest(o, items)
	result, provErr := s.Provider.Issue(ctx, req)

	status := result.Status
	if status == "" {
		status = domain.InvoiceStatusPending
	}
	payload := result.PayloadJSON
	if payload == "" {
		payload = "{}"
	}
	if provErr != nil {
		status = domain.InvoiceStatusFailed
		errPayload, _ := json.Marshal(map[string]string{"error": provErr.Error()})
		payload = string(errPayload)
	}

	now := time.Now().UTC()
	var issuedAt any
	if status == domain.InvoiceStatusIssued {
		issuedAt = now
	}
	_, err = s.DB.ExecContext(ctx, `
update invoices
set external_id=$1,
    status=$2,
    payload_json=$3::jsonb,
    cufe=$4,
    resolution_number=$5,
    resolution_prefix=$6,
    sequence_number=$7,
    xml_url=$8,
    pdf_url=$9,
    issued_at=coalesce(issued_at,$10),
    updated_at=$11
where order_id=$12
`,
		nullIfEmpty(result.ExternalID),
		string(status),
		payload,
		nullIfEmpty(result.CUFE),
		nullIfEmpty(result.ResolutionNumber),
		nullIfEmpty(result.ResolutionPrefix),
		nullIfZero(result.SequenceNumber),
		nullIfEmpty(result.XMLURL),
		nullIfEmpty(result.PDFURL),
		issuedAt,
		now,
		string(orderID),
	)
	if err != nil {
		return nil, err
	}

	if s.Audit != nil && actor != "" {
		auditPayload, _ := json.Marshal(map[string]string{
			"order_id":    string(orderID),
			"provider":    s.Provider.Name(),
			"external_id": result.ExternalID,
			"status":      string(status),
			"actor":       actor,
		})
		_ = s.Audit.Insert(ctx, s.DB, "invoice", invID, actor, "invoice.issue", string(auditPayload))
	}

	out, err := s.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if provErr != nil {
		return out, provErr
	}
	return out, nil
}

func buildIssueRequest(o *domain.Order, items []domain.OrderItem) IssueRequest {
	req := IssueRequest{
		OrderID:    o.ID,
		Currency:   o.Currency,
		TotalMinor: o.TotalMinorUnits,
		LegalID:    o.LegalID,
		FullName:   o.FullName,
		City:       o.City,
		Address:    o.AddressLine1,
	}
	for _, it := range items {
		req.LineItems = append(req.LineItems, IssueLineItem{
			SKU:       it.SKU,
			Name:      it.Name,
			Qty:       it.Qty,
			UnitMinor: it.UnitPriceMinorUnits,
			LineMinor: it.LineTotalMinorUnits,
		})
	}
	return req
}

func scanInvoice(row *sql.Row) (*domain.Invoice, error) {
	var inv domain.Invoice
	var (
		payload    sql.NullString
		issuedAt   sql.NullTime
		seqNumber  int64
		cufe       string
		resNumber  string
		resPrefix  string
		xmlURL     string
		pdfURL     string
	)
	err := row.Scan(
		&inv.ID, &inv.OrderID, &inv.Provider, &inv.ExternalID, &inv.Status,
		&payload,
		&cufe, &resNumber, &resPrefix, &seqNumber,
		&xmlURL, &pdfURL, &issuedAt,
		&inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if payload.Valid {
		inv.PayloadJSON = payload.String
	}
	inv.CUFE = cufe
	inv.ResolutionNumber = resNumber
	inv.ResolutionPrefix = resPrefix
	inv.SequenceNumber = seqNumber
	inv.XMLURL = xmlURL
	inv.PDFURL = pdfURL
	if issuedAt.Valid {
		t := issuedAt.Time
		inv.IssuedAt = &t
	}
	return &inv, nil
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
