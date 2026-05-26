package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) GetProduct(ctx context.Context, q Querier, id domain.ProductID) (*domain.Product, error) {
	row := q.QueryRowContext(ctx, `
select id, sku, name, brewery, style, abv, volume_ml, invima_reg, image_url, description, ibu, is_active, created_at, updated_at
from products
where id = $1
`, string(id))

	p, err := scanProduct(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *SQLRepository) GetProductBySKU(ctx context.Context, q Querier, sku string) (*domain.Product, error) {
	row := q.QueryRowContext(ctx, `
select id, sku, name, brewery, style, abv, volume_ml, invima_reg, image_url, description, ibu, is_active, created_at, updated_at
from products
where sku = $1
`, sku)

	p, err := scanProduct(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *SQLRepository) ListActiveProducts(ctx context.Context, q Querier, filter ListCatalogFilter) (ListCatalogResult, error) {
	currency := strings.TrimSpace(filter.Currency)
	if currency == "" {
		currency = "COP"
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var where []string
	var args []any
	args = append(args, currency)
	argN := 2

	add := func(clause string, val any) {
		where = append(where, fmt.Sprintf(clause, argN))
		args = append(args, val)
		argN++
	}

	if q := strings.TrimSpace(filter.Query); q != "" {
		pat := "%" + q + "%"
		where = append(where, fmt.Sprintf("(p.sku ilike $%d or p.name ilike $%d or p.brewery ilike $%d)", argN, argN+1, argN+2))
		args = append(args, pat, pat, pat)
		argN += 3
	}
	if style := strings.TrimSpace(filter.Style); style != "" {
		add("p.style ilike $%d", "%"+style+"%")
	}
	if brewery := strings.TrimSpace(filter.Brewery); brewery != "" {
		add("p.brewery ilike $%d", "%"+brewery+"%")
	}
	if filter.ABVMin != nil {
		add("p.abv >= $%d", *filter.ABVMin)
	}
	if filter.ABVMax != nil {
		add("p.abv <= $%d", *filter.ABVMax)
	}

	whereSQL := "p.is_active = true"
	if len(where) > 0 {
		whereSQL += " and " + strings.Join(where, " and ")
	}

	orderBy := "p.sku asc"
	if filter.Featured {
		// Pilot-grade "destacados": no schema flag yet, just surface the
		// freshest active inventory first so the TUI's first paint never
		// looks stale. Tie-break by sku so the order is deterministic.
		orderBy = "p.created_at desc, p.sku asc"
	}

	fetchLimit := limit + 1
	args = append(args, fetchLimit, offset)

	query := fmt.Sprintf(`
select %s,
       pr.product_id, pr.currency, pr.amount_minor_units, pr.valid_from, pr.valid_through,
       greatest(0, coalesce(i.on_hand,0) - coalesce(i.reserved,0) - coalesce(i.safety_stock,0)) as available_qty
from products p
left join inventory i on i.product_id = p.id
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
where %s
order by %s
limit $%d offset $%d
`, productSelectCols, whereSQL, orderBy, argN, argN+1)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return ListCatalogResult{}, err
	}
	defer rows.Close()

	var out []ProductWithPrice
	for rows.Next() {
		var p domain.Product
		var imageURL sql.NullString
		var ibu sql.NullInt32
		var pr domain.Price
		var validThrough sql.NullTime
		var available int
		if err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &imageURL, &p.Description, &ibu, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&pr.ProductID, &pr.Currency, &pr.AmountMinorUnits, &pr.ValidFrom, &validThrough,
			&available,
		); err != nil {
			return ListCatalogResult{}, err
		}
		if imageURL.Valid {
			p.ImageURL = imageURL.String
		}
		if ibu.Valid {
			v := int(ibu.Int32)
			p.IBU = &v
		}
		if validThrough.Valid {
			t := validThrough.Time
			pr.ValidThrough = &t
		}
		out = append(out, ProductWithPrice{Product: p, Price: pr, AvailableQty: available})
	}
	if err := rows.Err(); err != nil {
		return ListCatalogResult{}, err
	}

	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return ListCatalogResult{Items: out, HasMore: hasMore}, nil
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
