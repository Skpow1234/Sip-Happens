package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"sip-happens/pkg/ids"
	"sip-happens/pkg/domain"
)

type Service interface {
	ListProducts(ctx context.Context, currency string, limit int) ([]ProductRow, error)
	UpsertProduct(ctx context.Context, in UpsertProductInput) (domain.ProductID, error)
	SetPrice(ctx context.Context, in SetPriceInput) error
	UpsertInventory(ctx context.Context, in UpsertInventoryInput) error
}

type SQLService struct {
	DB *sql.DB
}

type ProductRow struct {
	Product domain.Product
	Price   *domain.Price
}

type UpsertProductInput struct {
	SKU       string
	Name      string
	Brewery   string
	Style     string
	ABV       float64
	VolumeML  int
	InvimaReg string
	IsActive  bool
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

func (s *SQLService) ListProducts(ctx context.Context, currency string, limit int) ([]ProductRow, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("missing db")
	}
	if currency == "" {
		currency = "COP"
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	rows, err := s.DB.QueryContext(ctx, `
select p.id, p.sku, p.name, p.brewery, p.style, p.abv, p.volume_ml, p.invima_reg, p.is_active, p.created_at, p.updated_at,
       pr.product_id, pr.currency, pr.amount_minor_units, pr.valid_from, pr.valid_through
from products p
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
`, currency, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProductRow
	for rows.Next() {
		var p domain.Product
		var pr domain.Price
		var prProductID sql.NullString
		var prCurrency sql.NullString
		var prAmount sql.NullInt64
		var prValidFrom sql.NullTime
		var prValidThrough sql.NullTime

		if err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&prProductID, &prCurrency, &prAmount, &prValidFrom, &prValidThrough,
		); err != nil {
			return nil, err
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

func (s *SQLService) UpsertProduct(ctx context.Context, in UpsertProductInput) (domain.ProductID, error) {
	if s.DB == nil {
		return "", fmt.Errorf("missing db")
	}
	if in.SKU == "" || in.Name == "" || in.Brewery == "" || in.Style == "" || in.VolumeML <= 0 || in.InvimaReg == "" {
		return "", domain.ErrInvalidInput
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
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`, id, in.SKU, in.Name, in.Brewery, in.Style, in.ABV, in.VolumeML, in.InvimaReg, in.IsActive, now, now)
		if err != nil {
			return "", err
		}
		return domain.ProductID(id), nil
	}

	_, err = s.DB.ExecContext(ctx, `
update products
set name=$1, brewery=$2, style=$3, abv=$4, volume_ml=$5, invima_reg=$6, is_active=$7, updated_at=$8
where id=$9
`, in.Name, in.Brewery, in.Style, in.ABV, in.VolumeML, in.InvimaReg, in.IsActive, now, id)
	if err != nil {
		return "", err
	}
	return domain.ProductID(id), nil
}

func (s *SQLService) SetPrice(ctx context.Context, in SetPriceInput) error {
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
	return err
}

func (s *SQLService) UpsertInventory(ctx context.Context, in UpsertInventoryInput) error {
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
	return err
}

