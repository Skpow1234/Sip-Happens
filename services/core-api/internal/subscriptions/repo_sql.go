package subscriptions

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"sip-happens/pkg/domain"
)

type SQLRepository struct{}

func NewSQLRepository() *SQLRepository { return &SQLRepository{} }

const subscriptionSelectCols = `
id, customer_id, product_id, qty, currency, interval, unit_amount_minor_units, status,
stripe_subscription_id, stripe_subscription_item_id, address_id,
current_period_start, current_period_end, cancelled_at, created_at, updated_at
`

func scanSubscription(row interface {
	Scan(dest ...any) error
}) (domain.Subscription, error) {
	var sub domain.Subscription
	var interval, status string
	var stripeSubID, stripeItemID, addrID sql.NullString
	var periodStart, periodEnd, cancelledAt sql.NullTime
	if err := row.Scan(
		&sub.ID, &sub.CustomerID, &sub.ProductID, &sub.Qty, &sub.Currency, &interval,
		&sub.UnitAmountMinorUnits, &status,
		&stripeSubID, &stripeItemID, &addrID,
		&periodStart, &periodEnd, &cancelledAt,
		&sub.CreatedAt, &sub.UpdatedAt,
	); err != nil {
		return domain.Subscription{}, err
	}
	sub.Interval = domain.SubscriptionInterval(interval)
	sub.Status = domain.SubscriptionStatus(status)
	if stripeSubID.Valid {
		sub.StripeSubscriptionID = stripeSubID.String
	}
	if stripeItemID.Valid {
		sub.StripeSubscriptionItemID = stripeItemID.String
	}
	if addrID.Valid {
		sub.AddressID = domain.AddressID(addrID.String)
	}
	if periodStart.Valid {
		t := periodStart.Time
		sub.CurrentPeriodStart = &t
	}
	if periodEnd.Valid {
		t := periodEnd.Time
		sub.CurrentPeriodEnd = &t
	}
	if cancelledAt.Valid {
		t := cancelledAt.Time
		sub.CancelledAt = &t
	}
	return sub, nil
}

func (r *SQLRepository) ListByCustomer(ctx context.Context, q Querier, customerID domain.CustomerID, limit, offset int) (ListPage, error) {
	if limit < 1 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := q.QueryContext(ctx, `
select `+subscriptionSelectCols+`
from customer_subscriptions
where customer_id = $1
order by created_at desc
limit $2 offset $3
`, string(customerID), limit+1, offset)
	if err != nil {
		return ListPage{}, err
	}
	defer rows.Close()

	var out []domain.Subscription
	for rows.Next() {
		sub, err := scanSubscription(rows)
		if err != nil {
			return ListPage{}, err
		}
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		return ListPage{}, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return ListPage{Subscriptions: out, HasMore: hasMore}, nil
}

func (r *SQLRepository) Get(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.SubscriptionID) (*domain.Subscription, error) {
	row := q.QueryRowContext(ctx, `
select `+subscriptionSelectCols+`
from customer_subscriptions
where customer_id = $1 and id = $2
`, string(customerID), string(id))
	sub, err := scanSubscription(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SQLRepository) Create(ctx context.Context, q Querier, sub domain.Subscription) error {
	var addr any
	if sub.AddressID != "" {
		addr = string(sub.AddressID)
	}
	_, err := q.ExecContext(ctx, `
insert into customer_subscriptions (
  id, customer_id, product_id, qty, currency, interval, unit_amount_minor_units, status,
  address_id, created_at, updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
`, string(sub.ID), string(sub.CustomerID), string(sub.ProductID), sub.Qty, sub.Currency,
		string(sub.Interval), sub.UnitAmountMinorUnits, string(sub.Status),
		addr, sub.CreatedAt, sub.UpdatedAt)
	return err
}

func (r *SQLRepository) Update(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.SubscriptionID, in UpdateInput) (*domain.Subscription, error) {
	cur, err := r.Get(ctx, q, customerID, id)
	if err != nil {
		return nil, err
	}
	if cur.Status == domain.SubscriptionStatusCancelled {
		return nil, domain.ErrConflict
	}

	qty := cur.Qty
	if in.Qty != nil {
		if *in.Qty <= 0 {
			return nil, domain.ErrInvalidInput
		}
		qty = *in.Qty
	}

	addr := cur.AddressID
	if in.AddressID != nil {
		addr = *in.AddressID
	}

	var addrParam any
	if addr != "" {
		addrParam = string(addr)
	}

	res, err := q.ExecContext(ctx, `
update customer_subscriptions
set qty = $3, address_id = $4, updated_at = now()
where customer_id = $1 and id = $2 and status <> 'CANCELLED'
`, string(customerID), string(id), qty, addrParam)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, domain.ErrNotFound
	}
	return r.Get(ctx, q, customerID, id)
}

func (r *SQLRepository) Cancel(ctx context.Context, q Querier, customerID domain.CustomerID, id domain.SubscriptionID) (*domain.Subscription, error) {
	cur, err := r.Get(ctx, q, customerID, id)
	if err != nil {
		return nil, err
	}
	if cur.Status == domain.SubscriptionStatusCancelled {
		return cur, nil
	}

	now := time.Now()
	res, err := q.ExecContext(ctx, `
update customer_subscriptions
set status = 'CANCELLED', cancelled_at = $3, updated_at = now()
where customer_id = $1 and id = $2 and status <> 'CANCELLED'
`, string(customerID), string(id), now)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, domain.ErrNotFound
	}
	return r.Get(ctx, q, customerID, id)
}

// ResolveSubscription looks up by public id, internal UUID, or raw id for a customer.
func ResolveSubscription(ctx context.Context, repo Repository, q Querier, customerID domain.CustomerID, ref string) (*domain.Subscription, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, domain.ErrInvalidInput
	}
	if id, ok := SubscriptionIDFromPublicID(ref); ok {
		return repo.Get(ctx, q, customerID, id)
	}
	return repo.Get(ctx, q, customerID, domain.SubscriptionID(ref))
}
