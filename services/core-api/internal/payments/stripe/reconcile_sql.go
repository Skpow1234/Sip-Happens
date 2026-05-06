package stripe

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/pkg/domain"
)

type ReconcileCandidate struct {
	OrderID          domain.OrderID
	PaymentIntentID  string
	CheckoutSessionID string
}

type SQLReconcileQueue struct {
	DB *sql.DB
}

func (q *SQLReconcileQueue) ListDue(ctx context.Context, now time.Time, limit int) ([]domain.OrderID, error) {
	rows, err := q.DB.QueryContext(ctx, `
select order_id
from stripe_reconcile_queue
where next_attempt_at <= $1
order by next_attempt_at asc
limit $2
`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.OrderID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, domain.OrderID(id))
	}
	return out, rows.Err()
}

func (q *SQLReconcileQueue) Upsert(ctx context.Context, orderID domain.OrderID, nextAttempt time.Time) error {
	_, err := q.DB.ExecContext(ctx, `
insert into stripe_reconcile_queue (order_id, next_attempt_at, attempts, updated_at)
values ($1,$2,0,now())
on conflict (order_id) do update
set next_attempt_at=excluded.next_attempt_at,
    updated_at=now()
`, string(orderID), nextAttempt)
	return err
}

func (q *SQLReconcileQueue) MarkAttempt(ctx context.Context, orderID domain.OrderID, nextAttempt time.Time, errMsg string) error {
	_, err := q.DB.ExecContext(ctx, `
update stripe_reconcile_queue
set attempts=attempts+1,
    next_attempt_at=$2,
    last_error=$3,
    updated_at=now()
where order_id=$1
`, string(orderID), nextAttempt, errMsg)
	return err
}

func (q *SQLReconcileQueue) Delete(ctx context.Context, orderID domain.OrderID) error {
	_, err := q.DB.ExecContext(ctx, `delete from stripe_reconcile_queue where order_id=$1`, string(orderID))
	return err
}

