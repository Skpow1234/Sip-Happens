package customers

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

func (r *SQLRepository) GetByID(ctx context.Context, q Querier, id domain.CustomerID) (*domain.Customer, error) {
	row := q.QueryRowContext(ctx, `
select id, ssh_fingerprint, email, phone, legal_id, full_name, birth_date,
       address_line1, neighborhood, city, created_at, updated_at
from customers where id=$1
`, string(id))
	var c domain.Customer
	var email, phone, legalID, fullName, addr, hood, city sql.NullString
	var birth sql.NullTime
	if err := row.Scan(
		&c.ID, &c.SSHFingerprint, &email, &phone, &legalID, &fullName, &birth,
		&addr, &hood, &city, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	c.Email = email.String
	c.Phone = phone.String
	c.LegalID = legalID.String
	c.FullName = fullName.String
	c.AddressLine1 = addr.String
	c.Neighborhood = hood.String
	c.City = city.String
	if birth.Valid {
		c.BirthDate = birth.Time
	}
	return &c, nil
}

func (r *SQLRepository) IDByFingerprint(ctx context.Context, q Querier, fp string) (domain.CustomerID, error) {
	row := q.QueryRowContext(ctx, `select id from customers where ssh_fingerprint=$1`, fp)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return domain.CustomerID(id), nil
}

func (r *SQLRepository) InsertByFingerprint(ctx context.Context, q Querier, id domain.CustomerID, fp string) error {
	_, err := q.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ($1,$2,now(),now())
on conflict (ssh_fingerprint) do nothing
`, string(id), fp)
	return err
}

func (r *SQLRepository) Update(ctx context.Context, q Querier, c *domain.Customer) error {
	var birth any
	if !c.BirthDate.IsZero() {
		birth = c.BirthDate
	}
	_, err := q.ExecContext(ctx, `
update customers
set city=$2, neighborhood=$3, address_line1=$4, phone=$5, legal_id=$6, full_name=$7,
    birth_date=$8, updated_at=now()
where id=$1
`, string(c.ID), c.City, c.Neighborhood, c.AddressLine1, c.Phone, c.LegalID, c.FullName, birth)
	return err
}

func (r *SQLRepository) SaveCheckoutProfile(ctx context.Context, q Querier, id domain.CustomerID, city, neighborhood, addr, phone, legalID, fullName string, birth any) error {
	_, err := q.ExecContext(ctx, `
update customers
set city=$2, neighborhood=$3, address_line1=$4, phone=$5, legal_id=$6, full_name=$7,
    birth_date=coalesce($8, birth_date), updated_at=now()
where id=$1
`, string(id), city, neighborhood, addr, phone, legalID, fullName, birth)
	return err
}

func (r *SQLRepository) LoadCart(ctx context.Context, q Querier, customerID domain.CustomerID) ([]CartLine, error) {
	var raw []byte
	err := q.QueryRowContext(ctx, `
select items_json from customer_carts where customer_id=$1
`, string(customerID)).Scan(&raw)
	if err == sql.ErrNoRows {
		return []CartLine{}, nil
	}
	if err != nil {
		return nil, err
	}
	var items []CartLine
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *SQLRepository) StripeCustomerID(ctx context.Context, q Querier, id domain.CustomerID) (string, error) {
	row := q.QueryRowContext(ctx, `
select stripe_customer_id from customers where id=$1
`, string(id))
	var stripeID sql.NullString
	if err := row.Scan(&stripeID); err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return strings.TrimSpace(stripeID.String), nil
}

func (r *SQLRepository) SetStripeCustomerID(ctx context.Context, q Querier, id domain.CustomerID, stripeCustomerID string) error {
	stripeCustomerID = strings.TrimSpace(stripeCustomerID)
	if stripeCustomerID == "" {
		return domain.ErrInvalidInput
	}
	res, err := q.ExecContext(ctx, `
update customers set stripe_customer_id=$2, updated_at=now() where id=$1
`, string(id), stripeCustomerID)
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

func (r *SQLRepository) SaveCart(ctx context.Context, q Querier, customerID domain.CustomerID, itemsJSON []byte) error {
	_, err := q.ExecContext(ctx, `
insert into customer_carts (customer_id, items_json, updated_at)
values ($1, $2::jsonb, now())
on conflict (customer_id) do update
set items_json = excluded.items_json, updated_at = now()
`, string(customerID), string(itemsJSON))
	return err
}

func (r *SQLRepository) RecordConsent(ctx context.Context, q Querier, customerID domain.CustomerID, purpose string) error {
	_, err := q.ExecContext(ctx, `
insert into customer_consents (customer_id, purpose, granted_at, revoked_at)
values ($1, $2, now(), null)
on conflict (customer_id, purpose) do update
set granted_at = now(), revoked_at = null
`, string(customerID), purpose)
	return err
}

func (r *SQLRepository) InsertAgeVerification(ctx context.Context, q Querier, id string, customerID domain.CustomerID, orderID domain.OrderID, birthDate, affirmedAt time.Time, method string) error {
	_, err := q.ExecContext(ctx, `
insert into age_verifications (id, customer_id, order_id, birth_date, affirmed_at, method, created_at)
values ($1, $2, $3, $4::date, $5, $6, now())
`, id, string(customerID), string(orderID), birthDate.Format("2006-01-02"), affirmedAt, method)
	return err
}
