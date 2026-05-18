package addresses

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

const addressSelectCols = `
id, customer_id, label, full_name, phone, legal_id,
address_line1, neighborhood, city, city_normalized, is_default, created_at, updated_at
`

func scanAddress(row interface {
	Scan(dest ...any) error
}) (domain.CustomerAddress, error) {
	var a domain.CustomerAddress
	if err := row.Scan(
		&a.ID, &a.CustomerID, &a.Label, &a.FullName, &a.Phone, &a.LegalID,
		&a.AddressLine1, &a.Neighborhood, &a.City, &a.CityNormalized, &a.IsDefault,
		&a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return domain.CustomerAddress{}, err
	}
	return a, nil
}

func (r *SQLRepository) ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID) ([]domain.CustomerAddress, error) {
	rows, err := q.QueryContext(ctx, `
select `+addressSelectCols+`
from customer_addresses
where customer_id = $1
order by is_default desc, created_at asc
`, string(customerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CustomerAddress
	for rows.Next() {
		a, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *SQLRepository) Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.AddressID) (*domain.CustomerAddress, error) {
	row := q.QueryRowContext(ctx, `
select `+addressSelectCols+`
from customer_addresses
where customer_id = $1 and id = $2
`, string(customerID), string(id))
	a, err := scanAddress(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *SQLRepository) Create(ctx context.Context, q Querier, addr domain.CustomerAddress) error {
	if addr.IsDefault {
		if _, err := q.ExecContext(ctx, `
update customer_addresses set is_default = false, updated_at = now()
where customer_id = $1 and is_default = true
`, string(addr.CustomerID)); err != nil {
			return err
		}
	}
	_, err := q.ExecContext(ctx, `
insert into customer_addresses (
  id, customer_id, label, full_name, phone, legal_id,
  address_line1, neighborhood, city, city_normalized, is_default, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
`, string(addr.ID), string(addr.CustomerID), addr.Label, addr.FullName, addr.Phone, addr.LegalID,
		addr.AddressLine1, addr.Neighborhood, addr.City, addr.CityNormalized, addr.IsDefault,
		addr.CreatedAt, addr.UpdatedAt)
	return err
}

func (r *SQLRepository) Delete(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.AddressID) error {
	res, err := q.ExecContext(ctx, `
delete from customer_addresses where customer_id = $1 and id = $2
`, string(customerID), string(id))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SQLRepository) SetDefault(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.AddressID) error {
	res, err := q.ExecContext(ctx, `
update customer_addresses set is_default = false, updated_at = now()
where customer_id = $1 and is_default = true and id <> $2
`, string(customerID), string(id))
	if err != nil {
		return err
	}
	_ = res
	res, err = q.ExecContext(ctx, `
update customer_addresses set is_default = true, updated_at = now()
where customer_id = $1 and id = $2
`, string(customerID), string(id))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ResolveAddress looks up by public id, internal UUID, or raw id for a customer.
func ResolveAddress(ctx context.Context, repo Repository, q Querier, customerID domain.CustomerID, ref string) (*domain.CustomerAddress, error) {
	if id, ok := AddressIDFromPublicID(ref); ok {
		return repo.Get(ctx, q, customerID, id)
	}
	return repo.Get(ctx, q, customerID, domain.AddressID(ref))
}
