package admin

import (
	"context"
	"fmt"
	"time"

	"sip-happens/pkg/domain"
)

type InventoryRow struct {
	ProductID    domain.ProductID `json:"product_id"`
	SKU          string           `json:"sku"`
	Name         string           `json:"name"`
	OnHand       int              `json:"on_hand"`
	Reserved     int              `json:"reserved"`
	SafetyStock  int              `json:"safety_stock"`
	AvailableQty int              `json:"available_qty"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type ListInventoryInput struct {
	ProductID domain.ProductID
	SKU       string
	Limit     int
	Offset    int
}

type ActiveReservationRow struct {
	OrderID     domain.OrderID   `json:"order_id"`
	DisplayRef  string           `json:"display_ref,omitempty"`
	ProductID   domain.ProductID `json:"product_id"`
	SKU         string           `json:"sku"`
	Qty         int              `json:"qty"`
	Status      string           `json:"status"`
	OrderStatus string           `json:"order_status"`
	ExpiresAt   *time.Time       `json:"expires_at,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}

type ListActiveReservationsInput struct {
	ProductID domain.ProductID
	SKU       string
	Limit     int
	Offset    int
}

func (s *SQLService) ListInventory(ctx context.Context, in ListInventoryInput) ([]InventoryRow, error) {
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

	var productID any
	if in.ProductID != "" {
		productID = string(in.ProductID)
	}
	sku := in.SKU

	rows, err := s.DB.QueryContext(ctx, `
select i.product_id, p.sku, p.name, i.on_hand, i.reserved, i.safety_stock, i.updated_at
from inventory i
join products p on p.id = i.product_id
where ($1::text is null or i.product_id = $1)
  and ($2::text = '' or p.sku = $2)
order by p.sku asc
limit $3 offset $4
`, productID, sku, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InventoryRow
	for rows.Next() {
		var row InventoryRow
		if err := rows.Scan(
			&row.ProductID,
			&row.SKU,
			&row.Name,
			&row.OnHand,
			&row.Reserved,
			&row.SafetyStock,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		inv := domain.Inventory{
			ProductID:   row.ProductID,
			OnHand:      row.OnHand,
			Reserved:    row.Reserved,
			SafetyStock: row.SafetyStock,
		}
		row.AvailableQty = inv.Available()
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *SQLService) ListActiveReservations(ctx context.Context, in ListActiveReservationsInput) ([]ActiveReservationRow, error) {
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

	var productID any
	if in.ProductID != "" {
		productID = string(in.ProductID)
	}
	sku := in.SKU

	rows, err := s.DB.QueryContext(ctx, `
select r.order_id, coalesce(o.display_ref, ''), r.product_id, p.sku, r.qty, r.status, o.status,
       o.expires_at, r.created_at
from inventory_reservations r
join products p on p.id = r.product_id
join orders o on o.id = r.order_id
where r.status = 'ACTIVE'
  and ($1::text is null or r.product_id = $1)
  and ($2::text = '' or p.sku = $2)
order by r.created_at desc
limit $3 offset $4
`, productID, sku, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ActiveReservationRow
	for rows.Next() {
		var row ActiveReservationRow
		var expiresAt *time.Time
		if err := rows.Scan(
			&row.OrderID,
			&row.DisplayRef,
			&row.ProductID,
			&row.SKU,
			&row.Qty,
			&row.Status,
			&row.OrderStatus,
			&expiresAt,
			&row.CreatedAt,
		); err != nil {
			return nil, err
		}
		row.ExpiresAt = expiresAt
		out = append(out, row)
	}
	return out, rows.Err()
}
