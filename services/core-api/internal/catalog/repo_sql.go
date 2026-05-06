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

