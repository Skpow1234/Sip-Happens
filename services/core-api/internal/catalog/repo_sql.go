package catalog

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) GetProduct(ctx context.Context, q Querier, id domain.ProductID) (*domain.Product, error) {
	row := q.QueryRowContext(ctx, `
select id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at
from products
where id = $1
`, string(id))

	var p domain.Product
	if err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *SQLRepository) GetProductBySKU(ctx context.Context, q Querier, sku string) (*domain.Product, error) {
	row := q.QueryRowContext(ctx, `
select id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at
from products
where sku = $1
`, sku)

	var p domain.Product
	if err := row.Scan(&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *SQLRepository) ListActiveProducts(ctx context.Context, q Querier, currency string, limit int) ([]ProductWithPrice, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := q.QueryContext(ctx, `
select p.id, p.sku, p.name, p.brewery, p.style, p.abv, p.volume_ml, p.invima_reg, p.is_active, p.created_at, p.updated_at,
       pr.product_id, pr.currency, pr.amount_minor_units, pr.valid_from, pr.valid_through
from products p
join lateral (
  select product_id, currency, amount_minor_units, valid_from, valid_through
  from prices
  where product_id = p.id
    and currency = $1
    and valid_from <= now()
    and (valid_through is null or valid_through > now())
  order by valid_from desc
  limit 1
) pr on true
where p.is_active = true
order by p.sku asc
limit $2
`, currency, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProductWithPrice
	for rows.Next() {
		var p domain.Product
		var pr domain.Price
		var validThrough sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&pr.ProductID, &pr.Currency, &pr.AmountMinorUnits, &pr.ValidFrom, &validThrough,
		); err != nil {
			return nil, err
		}
		if validThrough.Valid {
			t := validThrough.Time
			pr.ValidThrough = &t
		}
		out = append(out, ProductWithPrice{Product: p, Price: pr})
	}
	return out, rows.Err()
}

func (r *SQLRepository) GetCurrentPrice(ctx context.Context, q Querier, productID domain.ProductID, currency string) (*domain.Price, error) {
	row := q.QueryRowContext(ctx, `
select product_id, currency, amount_minor_units, valid_from, valid_through
from prices
where product_id = $1
  and currency = $2
  and valid_from <= now()
  and (valid_through is null or valid_through > now())
order by valid_from desc
limit 1
`, string(productID), currency)

	var pr domain.Price
	var validThrough sql.NullTime
	if err := row.Scan(&pr.ProductID, &pr.Currency, &pr.AmountMinorUnits, &pr.ValidFrom, &validThrough); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	if validThrough.Valid {
		t := validThrough.Time
		pr.ValidThrough = &t
	}
	return &pr, nil
}

