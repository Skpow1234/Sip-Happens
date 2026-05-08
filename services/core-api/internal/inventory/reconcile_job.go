package inventory

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"sip-happens/pkg/domain"
)

type ReconcileJob struct {
	DB          *sql.DB
	Inventory   Repository
	BatchLimit  int
	ExpireGrace time.Duration
	Fix         bool
	Log         *log.Logger
}

// RunOnce checks for reservation leaks and reserved mismatches.
func (j *ReconcileJob) RunOnce(ctx context.Context, now time.Time) (fixed int, flagged int, err error) {
	if j.DB == nil {
		return 0, 0, errors.New("missing db")
	}
	if j.Inventory == nil {
		return 0, 0, errors.New("missing repositories")
	}
	if j.BatchLimit <= 0 {
		j.BatchLimit = 500
	}
	if j.ExpireGrace < 0 {
		j.ExpireGrace = 0
	}
	l := j.Log
	if l == nil {
		l = log.Default()
	}

	// 1) Reservation leaks: ACTIVE reservations on cancelled/expired orders.
	leaks, err := j.listLeakedReservations(ctx, now, j.BatchLimit)
	if err != nil {
		return 0, 0, err
	}
	for _, leak := range leaks {
		flagged++
		if !j.Fix {
			continue
		}
		n, err := j.fixLeak(ctx, now, leak)
		if err != nil {
			l.Printf("inventory reconcile: fix leak failed order_id=%s product_id=%s err=%v", leak.OrderID, leak.ProductID, err)
			continue
		}
		fixed += n
	}

	// 2) Reserved mismatches: inventory.reserved != sum(active reservation qty).
	mismatches, err := j.listReservedMismatches(ctx, j.BatchLimit)
	if err != nil {
		return fixed, flagged, err
	}
	for _, mm := range mismatches {
		flagged++
		if j.Fix {
			if err := j.fixReservedMismatch(ctx, now, mm); err != nil {
				l.Printf("inventory reconcile: fix mismatch failed product_id=%s err=%v", mm.ProductID, err)
				continue
			}
			fixed++
		} else {
			l.Printf("inventory reconcile: mismatch product_id=%s reserved=%d desired=%d", mm.ProductID, mm.Reserved, mm.Desired)
		}
	}

	return fixed, flagged, nil
}

type leakedReservation struct {
	OrderID    domain.OrderID
	ProductID  domain.ProductID
	Qty        int
	Reason     string
	ExpiresAt  *time.Time
}

func (j *ReconcileJob) listLeakedReservations(ctx context.Context, now time.Time, limit int) ([]leakedReservation, error) {
	rows, err := j.DB.QueryContext(ctx, `
select r.order_id, r.product_id, r.qty,
       o.status,
       o.expires_at,
       case
         when o.status = 'CANCELLED' then 'cancelled'
         when o.status = 'PENDING_PAYMENT' and o.expires_at is not null and o.expires_at <= $1 - make_interval(secs => $2) then 'expired'
         else 'unexpected'
       end as reason
from inventory_reservations r
join orders o on o.id = r.order_id
where r.status='ACTIVE'
  and (
    o.status = 'CANCELLED'
    or (o.status='PENDING_PAYMENT' and o.expires_at is not null and o.expires_at <= $1 - make_interval(secs => $2))
  )
order by o.updated_at asc
limit $3
`, now, int(j.ExpireGrace.Seconds()), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []leakedReservation
	for rows.Next() {
		var rr leakedReservation
		var status string
		var expires sql.NullTime
		if err := rows.Scan(&rr.OrderID, &rr.ProductID, &rr.Qty, &status, &expires, &rr.Reason); err != nil {
			return nil, err
		}
		if expires.Valid {
			t := expires.Time
			rr.ExpiresAt = &t
		}
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (j *ReconcileJob) fixLeak(ctx context.Context, now time.Time, leak leakedReservation) (int, error) {
	fixed := 0
	err := withTxSerializable(ctx, j.DB, func(tx *sql.Tx) error {
		// Re-check order status in-tx.
		row := tx.QueryRowContext(ctx, `select status, expires_at from orders where id=$1 for update`, string(leak.OrderID))
		var status string
		var expires sql.NullTime
		if err := row.Scan(&status, &expires); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		if domain.OrderStatus(status) == domain.OrderStatusPaid {
			// Paid orders should consume reservations; do not auto-fix here.
			return nil
		}

		// Ensure the reservation is still ACTIVE before touching inventory counters.
		res, err := tx.ExecContext(ctx, `
update inventory_reservations
set status='RELEASED', updated_at=now()
where order_id=$1 and product_id=$2 and status='ACTIVE'
`, string(leak.OrderID), string(leak.ProductID))
		if err != nil {
			return err
		}
		ra, _ := res.RowsAffected()
		if ra == 0 {
			return nil
		}
		fixed = 1

		inv, err := j.Inventory.LockForUpdate(ctx, tx, leak.ProductID)
		if err != nil {
			return err
		}
		inv.Reserved -= leak.Qty
		if inv.Reserved < 0 {
			inv.Reserved = 0
		}
		if err := j.Inventory.Update(ctx, tx, *inv); err != nil {
			return err
		}

		// If this was an expired pending order, cancel it to prevent payment race.
		if domain.OrderStatus(status) == domain.OrderStatusPendingPayment && expires.Valid && expires.Time.Before(now.Add(-j.ExpireGrace)) {
			_, _ = tx.ExecContext(ctx, `update orders set status='CANCELLED', updated_at=now() where id=$1 and status='PENDING_PAYMENT'`, string(leak.OrderID))
		}
		return nil
	})
	return fixed, err
}

type reservedMismatch struct {
	ProductID domain.ProductID
	Reserved  int
	Desired   int
}

func (j *ReconcileJob) listReservedMismatches(ctx context.Context, limit int) ([]reservedMismatch, error) {
	rows, err := j.DB.QueryContext(ctx, `
with desired as (
  select product_id, coalesce(sum(qty),0)::int as desired
  from inventory_reservations
  where status='ACTIVE'
  group by product_id
)
select i.product_id, i.reserved, coalesce(d.desired,0)
from inventory i
left join desired d on d.product_id = i.product_id
where i.reserved <> coalesce(d.desired,0)
order by i.updated_at asc
limit $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []reservedMismatch
	for rows.Next() {
		var mm reservedMismatch
		if err := rows.Scan(&mm.ProductID, &mm.Reserved, &mm.Desired); err != nil {
			return nil, err
		}
		out = append(out, mm)
	}
	return out, rows.Err()
}

func (j *ReconcileJob) fixReservedMismatch(ctx context.Context, now time.Time, mm reservedMismatch) error {
	return withTxSerializable(ctx, j.DB, func(tx *sql.Tx) error {
		inv, err := j.Inventory.LockForUpdate(ctx, tx, mm.ProductID)
		if err != nil {
			return err
		}
		// Recompute desired in-tx.
		row := tx.QueryRowContext(ctx, `
select coalesce(sum(qty),0)::int
from inventory_reservations
where product_id=$1 and status='ACTIVE'
`, string(mm.ProductID))
		desired := 0
		if err := row.Scan(&desired); err != nil {
			return err
		}
		inv.Reserved = desired
		return j.Inventory.Update(ctx, tx, *inv)
	})
}

func withTxSerializable(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

