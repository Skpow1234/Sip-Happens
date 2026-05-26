package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/pkg/redact"
	"sip-happens/services/core-api/internal/audit"
	"sip-happens/services/core-api/internal/payments"
)

type Service interface {
	ListProducts(ctx context.Context, in ListProductsInput) ([]ProductRow, error)
	UpsertProduct(ctx context.Context, actor string, in UpsertProductInput) (domain.ProductID, error)
	PutProductByID(ctx context.Context, actor string, productID domain.ProductID, in UpsertProductInput) error
	PatchProductByID(ctx context.Context, actor string, productID domain.ProductID, in PatchProductInput) error
	DeactivateProduct(ctx context.Context, actor string, productID domain.ProductID, reason string) error
	SetPrice(ctx context.Context, actor string, in SetPriceInput) error
	EndCurrentPrice(ctx context.Context, actor string, productID domain.ProductID, currency string, reason string) error
	UpsertInventory(ctx context.Context, actor string, in UpsertInventoryInput) error
	ListInventory(ctx context.Context, in ListInventoryInput) ([]InventoryRow, error)
	ListActiveReservations(ctx context.Context, in ListActiveReservationsInput) ([]ActiveReservationRow, error)

	ListAuditEvents(ctx context.Context, in ListAuditEventsInput) ([]domain.AuditEvent, error)

	// ListOrdersAdmin lists orders across customers (newest first) with optional filters.
	ListOrdersAdmin(ctx context.Context, in ListOrdersAdminInput) ([]domain.Order, error)

	// ListPaymentsForOrder returns payment rows for an order (oldest first).
	ListPaymentsForOrder(ctx context.Context, orderID domain.OrderID) ([]domain.Payment, error)

	// ListPaymentStatusHistoryForOrder returns normalized status transitions (oldest first).
	ListPaymentStatusHistoryForOrder(ctx context.Context, orderID domain.OrderID) ([]domain.PaymentStatusHistoryEntry, error)

	// RecordAudit appends an audit_events row (admin/support tooling).
	RecordAudit(ctx context.Context, actor, aggregateType, aggregateID, eventType, payloadJSON string) error

	// ListPriceHistory returns every (currency, valid_from) row in the prices
	// ledger for a product (newest valid_from first), joined with the
	// matching audit_events (price.set / price.ended) so callers can render
	// "who changed what when" without two round-trips. Cross-currency rows
	// are interleaved by time; consumers that need a per-currency timeline
	// can group client-side.
	ListPriceHistory(ctx context.Context, productID domain.ProductID) ([]PriceHistoryEntry, error)

	SetProductImageURL(ctx context.Context, actor string, productID domain.ProductID, imageURL string) error

	// ProductUpdatedAt returns the row version used for If-Match / ETag optimistic locking.
	ProductUpdatedAt(ctx context.Context, productID domain.ProductID) (time.Time, error)
}

type SQLService struct {
	DB *sql.DB
}

type ProductRow struct {
	Product domain.Product
	Price   *domain.Price
}

type ListProductsInput struct {
	Currency string
	Limit    int
	Offset   int
	Query    string
	IsActive *bool
}

type UpsertProductInput struct {
	SKU         string
	Name        string
	Brewery     string
	Style       string
	ABV         float64
	VolumeML    int
	InvimaReg   string
	Description string
	IBU         *int
	ImageURL    string
	IsActive    bool
}

type PatchProductInput struct {
	// Optional fields; when nil/zero-value + not present, no change.
	Name        *string  `json:"name"`
	Brewery     *string  `json:"brewery"`
	Style       *string  `json:"style"`
	ABV         *float64 `json:"abv"`
	VolumeML    *int     `json:"volume_ml"`
	InvimaReg   *string  `json:"invima_reg"`
	Description *string  `json:"description"`
	IBU         *int     `json:"ibu"`
	ImageURL    *string  `json:"image_url"`
	IsActive    *bool    `json:"is_active"`
}

// PriceHistoryEntry is a single row in the per-product price ledger,
// augmented with the most recent matching audit metadata.
//
// `ValidThrough` is nil for the currently-active period; otherwise it is
// the timestamp at which `EndCurrentPrice` (or a subsequent `SetPrice`)
// ended this slice. `SetBy` / `SetAt` come from the `price.set` audit row
// inserted alongside the price write. `EndedBy` / `EndedReason` are only
// populated for closed slices and come from the matching `price.ended`
// audit row (if any) — older slices that were superseded by a SetPrice
// rather than EndCurrentPrice will have a `ValidThrough` but no
// `EndedBy`/`EndedReason`.
type PriceHistoryEntry struct {
	Currency         string     `json:"currency"`
	AmountMinorUnits int64      `json:"amount_minor_units"`
	ValidFrom        time.Time  `json:"valid_from"`
	ValidThrough     *time.Time `json:"valid_through,omitempty"`
	SetBy            string     `json:"set_by,omitempty"`
	SetAt            *time.Time `json:"set_at,omitempty"`
	EndedBy          string     `json:"ended_by,omitempty"`
	EndedReason      string     `json:"ended_reason,omitempty"`
}

type SetPriceInput struct {
	ProductID        domain.ProductID
	Currency         string
	AmountMinorUnits int64
}

type UpsertInventoryInput struct {
	ProductID   domain.ProductID
	OnHand      int
	SafetyStock int
}

type ListAuditEventsInput struct {
	AggregateType string
	AggregateID   string
	OrderID       string
	Limit         int
	Offset        int
}

type ListOrdersAdminInput struct {
	Status      string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Limit       int
	Offset      int
}

func (s *SQLService) ListProducts(ctx context.Context, in ListProductsInput) ([]ProductRow, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}

	currency := in.Currency
	if currency == "" {
		currency = "COP"
	}
	limit := in.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	q := strings.TrimSpace(in.Query)
	var qLike any
	if q != "" {
		qLike = "%" + q + "%"
	}
	var active any
	if in.IsActive != nil {
		active = *in.IsActive
	}

	rows, err := s.DB.QueryContext(ctx, `
select p.id, p.sku, p.name, p.brewery, p.style, p.abv, p.volume_ml, p.invima_reg, p.image_url, p.description, p.ibu, p.is_active, p.created_at, p.updated_at,
       pr.product_id, pr.currency, pr.amount_minor_units, pr.valid_from, pr.valid_through
from products p
where ($3::text is null or p.sku ilike $3 or p.name ilike $3 or p.brewery ilike $3)
  and ($4::boolean is null or p.is_active = $4)
left join lateral (
  select product_id, currency, amount_minor_units, valid_from, valid_through
  from prices
  where product_id = p.id
    and currency = $1
    and valid_from <= now()
    and (valid_through is null or valid_through > now())
  order by valid_from desc
  limit 1
) pr on true
order by p.sku asc
limit $2
offset $5
`, currency, limit, qLike, active, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProductRow
	for rows.Next() {
		var p domain.Product
		var imageURL sql.NullString
		var ibu sql.NullInt32
		var pr domain.Price
		var prProductID sql.NullString
		var prCurrency sql.NullString
		var prAmount sql.NullInt64
		var prValidFrom sql.NullTime
		var prValidThrough sql.NullTime

		if err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &imageURL, &p.Description, &ibu, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&prProductID, &prCurrency, &prAmount, &prValidFrom, &prValidThrough,
		); err != nil {
			return nil, err
		}

		if imageURL.Valid {
			p.ImageURL = imageURL.String
		}
		if ibu.Valid {
			v := int(ibu.Int32)
			p.IBU = &v
		}
		var pricePtr *domain.Price
		if prProductID.Valid {
			pr.ProductID = domain.ProductID(prProductID.String)
			pr.Currency = prCurrency.String
			pr.AmountMinorUnits = prAmount.Int64
			pr.ValidFrom = prValidFrom.Time
			if prValidThrough.Valid {
				t := prValidThrough.Time
				pr.ValidThrough = &t
			}
			pricePtr = &pr
		}

		out = append(out, ProductRow{Product: p, Price: pricePtr})
	}
	return out, rows.Err()
}

func (s *SQLService) UpsertProduct(ctx context.Context, actor string, in UpsertProductInput) (domain.ProductID, error) {
	if s.DB == nil {
		return "", fmt.Errorf("missing db")
	}
	if in.SKU == "" || in.Name == "" || in.Brewery == "" || in.Style == "" || in.VolumeML <= 0 {
		return "", domain.ErrInvalidInput
	}
	if err := validateProductPublish(in.InvimaReg, in.IsActive); err != nil {
		return "", err
	}

	now := time.Now()

	// Try to fetch existing id by sku.
	var existingID string
	err := s.DB.QueryRowContext(ctx, `select id from products where sku=$1`, in.SKU).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	id := existingID
	if id == "" {
		u, err := ids.UUIDv4()
		if err != nil {
			return "", err
		}
		id = u
		_, err = s.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, image_url, description, ibu, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
`, id, in.SKU, in.Name, in.Brewery, in.Style, in.ABV, in.VolumeML, in.InvimaReg, nullString(in.ImageURL), in.Description, nullInt(in.IBU), in.IsActive, now, now)
		if err != nil {
			return "", err
		}
		_ = s.insertAudit(ctx, "product", id, actor, "product.created", fmt.Sprintf(`{"sku":%q}`, in.SKU))
		return domain.ProductID(id), nil
	}

	_, err = s.DB.ExecContext(ctx, `
update products
set name=$1, brewery=$2, style=$3, abv=$4, volume_ml=$5, invima_reg=$6, image_url=$7, description=$8, ibu=$9, is_active=$10, updated_at=$11
where id=$12
`, in.Name, in.Brewery, in.Style, in.ABV, in.VolumeML, in.InvimaReg, nullString(in.ImageURL), in.Description, nullInt(in.IBU), in.IsActive, now, id)
	if err != nil {
		return "", err
	}
	_ = s.insertAudit(ctx, "product", id, actor, "product.updated", fmt.Sprintf(`{"sku":%q}`, in.SKU))
	return domain.ProductID(id), nil
}

func (s *SQLService) PutProductByID(ctx context.Context, actor string, productID domain.ProductID, in UpsertProductInput) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if productID == "" {
		return domain.ErrInvalidInput
	}
	if in.SKU == "" || in.Name == "" || in.Brewery == "" || in.Style == "" || in.VolumeML <= 0 {
		return domain.ErrInvalidInput
	}
	if err := validateProductPublish(in.InvimaReg, in.IsActive); err != nil {
		return err
	}
	now := time.Now()
	res, err := s.DB.ExecContext(ctx, `
update products
set sku=$1, name=$2, brewery=$3, style=$4, abv=$5, volume_ml=$6, invima_reg=$7, image_url=$8, description=$9, ibu=$10, is_active=$11, updated_at=$12
where id=$13
`, in.SKU, in.Name, in.Brewery, in.Style, in.ABV, in.VolumeML, in.InvimaReg, nullString(in.ImageURL), in.Description, nullInt(in.IBU), in.IsActive, now, string(productID))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	_ = s.insertAudit(ctx, "product", string(productID), actor, "product.put", fmt.Sprintf(`{"sku":%q}`, in.SKU))
	return nil
}

func (s *SQLService) PatchProductByID(ctx context.Context, actor string, productID domain.ProductID, in PatchProductInput) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if productID == "" {
		return domain.ErrInvalidInput
	}
	// Read existing.
	row := s.DB.QueryRowContext(ctx, `
select sku, name, brewery, style, abv, volume_ml, invima_reg, coalesce(image_url,''), description, ibu, is_active
from products where id=$1
`, string(productID))
	var cur UpsertProductInput
	var ibu sql.NullInt32
	if err := row.Scan(&cur.SKU, &cur.Name, &cur.Brewery, &cur.Style, &cur.ABV, &cur.VolumeML, &cur.InvimaReg, &cur.ImageURL, &cur.Description, &ibu, &cur.IsActive); err != nil {
		if err == sql.ErrNoRows {
			return domain.ErrNotFound
		}
		return err
	}
	if ibu.Valid {
		v := int(ibu.Int32)
		cur.IBU = &v
	}

	if in.Name != nil {
		cur.Name = strings.TrimSpace(*in.Name)
	}
	if in.Brewery != nil {
		cur.Brewery = strings.TrimSpace(*in.Brewery)
	}
	if in.Style != nil {
		cur.Style = strings.TrimSpace(*in.Style)
	}
	if in.ABV != nil {
		cur.ABV = *in.ABV
	}
	if in.VolumeML != nil {
		cur.VolumeML = *in.VolumeML
	}
	if in.InvimaReg != nil {
		cur.InvimaReg = strings.TrimSpace(*in.InvimaReg)
	}
	if in.IsActive != nil {
		cur.IsActive = *in.IsActive
	}
	if in.ImageURL != nil {
		cur.ImageURL = strings.TrimSpace(*in.ImageURL)
	}
	if in.Description != nil {
		cur.Description = strings.TrimSpace(*in.Description)
	}
	if in.IBU != nil {
		cur.IBU = in.IBU
	}

	// Validate required invariants after patch.
	if cur.SKU == "" || cur.Name == "" || cur.Brewery == "" || cur.Style == "" || cur.VolumeML <= 0 {
		return domain.ErrInvalidInput
	}
	if err := validateProductPublish(cur.InvimaReg, cur.IsActive); err != nil {
		return err
	}

	now := time.Now()
	_, err := s.DB.ExecContext(ctx, `
	update products
set name=$1, brewery=$2, style=$3, abv=$4, volume_ml=$5, invima_reg=$6, image_url=$7, description=$8, ibu=$9, is_active=$10, updated_at=$11
where id=$12
`, cur.Name, cur.Brewery, cur.Style, cur.ABV, cur.VolumeML, cur.InvimaReg, nullString(cur.ImageURL), cur.Description, nullInt(cur.IBU), cur.IsActive, now, string(productID))
	if err != nil {
		return err
	}
	_ = s.insertAudit(ctx, "product", string(productID), actor, "product.patched", `{}`)
	return nil
}

func (s *SQLService) ProductUpdatedAt(ctx context.Context, productID domain.ProductID) (time.Time, error) {
	if s.DB == nil {
		return time.Time{}, fmt.Errorf("missing db")
	}
	if productID == "" {
		return time.Time{}, domain.ErrInvalidInput
	}
	var updatedAt time.Time
	err := s.DB.QueryRowContext(ctx, `select updated_at from products where id=$1`, string(productID)).Scan(&updatedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, domain.ErrNotFound
	}
	return updatedAt, err
}

func (s *SQLService) SetProductImageURL(ctx context.Context, actor string, productID domain.ProductID, imageURL string) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if productID == "" {
		return domain.ErrInvalidInput
	}
	imageURL = strings.TrimSpace(imageURL)
	res, err := s.DB.ExecContext(ctx, `update products set image_url=$1, updated_at=now() where id=$2`, nullString(imageURL), string(productID))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	_ = s.insertAudit(ctx, "product", string(productID), actor, "product.image_updated", fmt.Sprintf(`{"image_url":%q}`, imageURL))
	return nil
}

func nullString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func (s *SQLService) DeactivateProduct(ctx context.Context, actor string, productID domain.ProductID, reason string) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if productID == "" {
		return domain.ErrInvalidInput
	}
	res, err := s.DB.ExecContext(ctx, `update products set is_active=false, updated_at=now() where id=$1`, string(productID))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	_ = s.insertAudit(ctx, "product", string(productID), actor, "product.deactivated", fmt.Sprintf(`{"reason":%q}`, reason))
	return nil
}

func (s *SQLService) SetPrice(ctx context.Context, actor string, in SetPriceInput) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if in.ProductID == "" || in.Currency == "" || in.AmountMinorUnits <= 0 {
		return domain.ErrInvalidInput
	}

	now := time.Now()
	_, err := s.DB.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,$2,$3,$4)
`, string(in.ProductID), in.Currency, in.AmountMinorUnits, now)
	if err != nil {
		return err
	}
	_ = s.insertAudit(ctx, "product", string(in.ProductID), actor, "price.set", fmt.Sprintf(`{"currency":%q,"amount_minor_units":%d}`, in.Currency, in.AmountMinorUnits))
	return nil
}

func (s *SQLService) EndCurrentPrice(ctx context.Context, actor string, productID domain.ProductID, currency string, reason string) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if productID == "" {
		return domain.ErrInvalidInput
	}
	if currency == "" {
		currency = "COP"
	}
	res, err := s.DB.ExecContext(ctx, `
update prices
set valid_through=now()
where product_id=$1
  and currency=$2
  and valid_from <= now()
  and (valid_through is null or valid_through > now())
`, string(productID), currency)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	_ = s.insertAudit(ctx, "product", string(productID), actor, "price.ended", fmt.Sprintf(`{"currency":%q,"reason":%q}`, currency, reason))
	return nil
}

// ListPriceHistory aggregates the bi-temporal `prices` ledger with the
// matching audit rows. We do this in two queries (prices, then audit)
// rather than a big SQL join because audit payload is a redacted JSON
// blob — the audit timestamps line up with the price `valid_from` /
// `valid_through` ±1 ms in practice (same Go `time.Now()` value, just
// re-marshalled), so we match in Go by (event_type, JSON.currency,
// JSON.amount_minor_units, nearest-prior created_at).
func (s *SQLService) ListPriceHistory(ctx context.Context, productID domain.ProductID) ([]PriceHistoryEntry, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	if productID == "" {
		return nil, domain.ErrInvalidInput
	}

	rows, err := s.DB.QueryContext(ctx, `
select currency, amount_minor_units, valid_from, valid_through
from prices
where product_id = $1
order by valid_from desc, currency asc
`, string(productID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PriceHistoryEntry, 0, 8)
	for rows.Next() {
		var e PriceHistoryEntry
		var vt sql.NullTime
		if err := rows.Scan(&e.Currency, &e.AmountMinorUnits, &e.ValidFrom, &vt); err != nil {
			return nil, err
		}
		if vt.Valid {
			t := vt.Time
			e.ValidThrough = &t
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}

	auditRows, err := s.DB.QueryContext(ctx, `
select actor, event_type, payload_json, created_at
from audit_events
where aggregate_type = 'product'
  and aggregate_id = $1
  and event_type in ('price.set','price.ended')
order by created_at asc
`, string(productID))
	if err != nil {
		return out, nil // best-effort: return prices without audit metadata
	}
	defer auditRows.Close()

	type auditRow struct {
		actor     string
		eventType string
		payload   string
		createdAt time.Time
	}
	var audits []auditRow
	for auditRows.Next() {
		var a auditRow
		if err := auditRows.Scan(&a.actor, &a.eventType, &a.payload, &a.createdAt); err != nil {
			return out, nil
		}
		audits = append(audits, a)
	}

	matchPayload := func(payload, currency string, amount int64) bool {
		// Loose substring match — price.set/price.ended payloads use a
		// small predictable format `{"currency":"COP","amount_minor_units":1000}`
		// (see SetPrice). We avoid JSON-unmarshalling here to stay tolerant
		// of post-redaction shape changes.
		if !strings.Contains(payload, fmt.Sprintf(`"currency":%q`, currency)) {
			return false
		}
		if amount > 0 && !strings.Contains(payload, fmt.Sprintf(`"amount_minor_units":%d`, amount)) {
			return false
		}
		return true
	}

	for i := range out {
		e := &out[i]
		// price.set: pick the row whose created_at is closest at-or-after valid_from.
		var bestSet *auditRow
		for j := range audits {
			a := &audits[j]
			if a.eventType != "price.set" {
				continue
			}
			if !matchPayload(a.payload, e.Currency, e.AmountMinorUnits) {
				continue
			}
			if a.createdAt.Before(e.ValidFrom.Add(-1 * time.Second)) {
				continue
			}
			if bestSet == nil || a.createdAt.Before(bestSet.createdAt) {
				cp := *a
				bestSet = &cp
			}
		}
		if bestSet != nil {
			e.SetBy = bestSet.actor
			t := bestSet.createdAt
			e.SetAt = &t
		}

		if e.ValidThrough != nil {
			vt := *e.ValidThrough
			var bestEnd *auditRow
			for j := range audits {
				a := &audits[j]
				if a.eventType != "price.ended" {
					continue
				}
				if !strings.Contains(a.payload, fmt.Sprintf(`"currency":%q`, e.Currency)) {
					continue
				}
				if a.createdAt.Before(vt.Add(-1 * time.Second)) {
					continue
				}
				if bestEnd == nil || a.createdAt.Before(bestEnd.createdAt) {
					cp := *a
					bestEnd = &cp
				}
			}
			if bestEnd != nil {
				e.EndedBy = bestEnd.actor
				// Extract reason from payload: format `{"currency":"COP","reason":"foo"}`.
				if idx := strings.Index(bestEnd.payload, `"reason":"`); idx >= 0 {
					rest := bestEnd.payload[idx+len(`"reason":"`):]
					if end := strings.IndexByte(rest, '"'); end >= 0 {
						e.EndedReason = rest[:end]
					}
				}
			}
		}
	}

	return out, nil
}

func (s *SQLService) UpsertInventory(ctx context.Context, actor string, in UpsertInventoryInput) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	if in.ProductID == "" || in.OnHand < 0 || in.SafetyStock < 0 {
		return domain.ErrInvalidInput
	}
	_, err := s.DB.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,$2,0,$3,now())
on conflict (product_id) do update
set on_hand=excluded.on_hand,
    safety_stock=excluded.safety_stock,
    updated_at=now()
`, string(in.ProductID), in.OnHand, in.SafetyStock)
	if err != nil {
		return err
	}
	_ = s.insertAudit(ctx, "product", string(in.ProductID), actor, "inventory.upserted", fmt.Sprintf(`{"on_hand":%d,"safety_stock":%d}`, in.OnHand, in.SafetyStock))
	return nil
}

func (s *SQLService) ListAuditEvents(ctx context.Context, in ListAuditEventsInput) ([]domain.AuditEvent, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	limit := in.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	orderID := strings.TrimSpace(in.OrderID)
	rows, err := s.DB.QueryContext(ctx, `
select aggregate_type, aggregate_id, actor, event_type, payload_json, created_at
from audit_events
where ($1::text is null or aggregate_type = $1)
  and ($2::text is null or aggregate_id = $2)
  and (
    $5::text is null
    or (aggregate_type = 'order' and aggregate_id = $5)
    or (
      payload_json is not null
      and payload_json <> ''
      and payload_json::jsonb->>'order_id' = $5
    )
  )
order by id desc
limit $3 offset $4
`, nullableStr(strings.TrimSpace(in.AggregateType)), nullableStr(strings.TrimSpace(in.AggregateID)), limit, offset, nullableStr(orderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(&e.AggregateType, &e.AggregateID, &e.Actor, &e.EventType, &e.PayloadJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.PayloadJSON = redact.JSON(e.PayloadJSON)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *SQLService) ListOrdersAdmin(ctx context.Context, in ListOrdersAdminInput) ([]domain.Order, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	limit := in.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}

	var status any
	if strings.TrimSpace(in.Status) != "" {
		status = strings.TrimSpace(in.Status)
	}
	from := nullableTime(in.CreatedFrom)
	to := nullableTime(in.CreatedTo)

	rows, err := s.DB.QueryContext(ctx, `
select id, display_ref, status, customer_id,
       subtotal_minor_units, shipping_minor_units, total_minor_units, currency,
       city, neighborhood, address_line1, phone, legal_id, full_name,
       stripe_checkout_session_id, stripe_payment_intent_id,
       expires_at, created_at, updated_at
from orders
where ($1::text is null or status = $1)
  and ($2::timestamptz is null or created_at >= $2)
  and ($3::timestamptz is null or created_at <= $3)
order by created_at desc
limit $4 offset $5
`, status, from, to, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		var o domain.Order
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&o.ID, &o.DisplayRef, &o.Status, &o.CustomerID,
			&o.SubtotalMinorUnits, &o.ShippingMinorUnits, &o.TotalMinorUnits, &o.Currency,
			&o.City, &o.Neighborhood, &o.AddressLine1, &o.Phone, &o.LegalID, &o.FullName,
			&o.StripeCheckoutSessionID, &o.StripePaymentIntentID,
			&expiresAt, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			o.ExpiresAt = &t
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *SQLService) ListPaymentStatusHistoryForOrder(ctx context.Context, orderID domain.OrderID) ([]domain.PaymentStatusHistoryEntry, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	return payments.NewSQLRepository().ListStatusHistoryForOrder(ctx, s.DB, orderID)
}

func (s *SQLService) RecordAudit(ctx context.Context, actor, aggregateType, aggregateID, eventType, payloadJSON string) error {
	if s.DB == nil {
		return fmt.Errorf("missing db")
	}
	return s.insertAudit(ctx, aggregateType, aggregateID, actor, eventType, payloadJSON)
}

func (s *SQLService) ListPaymentsForOrder(ctx context.Context, orderID domain.OrderID) ([]domain.Payment, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	if orderID == "" {
		return nil, domain.ErrInvalidInput
	}
	rows, err := s.DB.QueryContext(ctx, `
select id, order_id, provider, provider_ref, status, raw_payload, created_at, updated_at
from payments
where order_id = $1
order by created_at asc
`, string(orderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var raw sql.NullString
		if err := rows.Scan(
			&p.ID, &p.OrderID, &p.Provider, &p.ProviderRef, &p.Status, &raw,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if raw.Valid {
			p.RawPayload = redact.JSON(raw.String)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *SQLService) insertAudit(ctx context.Context, aggType, aggID, actor, eventType, payload string) error {
	if actor == "" {
		actor = "admin"
	}
	return audit.NewSQLStore().Insert(ctx, s.DB, aggType, aggID, actor, eventType, payload)
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
