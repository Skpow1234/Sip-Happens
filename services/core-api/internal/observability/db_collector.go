package observability

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DBStatsCollector exposes backlog gauges from Postgres on each scrape.
// Roadmap 143–144: expired pending-payment orders and inventory anomalies.
type DBStatsCollector struct {
	db *sql.DB

	outboxPendingDesc           *prometheus.Desc
	outboxOldestDesc            *prometheus.Desc
	ordersPendingDesc           *prometheus.Desc
	ordersOldestPendingDesc     *prometheus.Desc
	ordersPastExpiresDesc       *prometheus.Desc
	ordersPastExpiresOldestDesc *prometheus.Desc
	invNegativeCountersDesc     *prometheus.Desc
	invReservedMismatchDesc     *prometheus.Desc
	invReservationLeaksDesc     *prometheus.Desc
}

func NewDBStatsCollector(db *sql.DB) *DBStatsCollector {
	return &DBStatsCollector{
		db: db,
		outboxPendingDesc: prometheus.NewDesc(
			"sip_happens_outbox_pending_events",
			"Outbox events with status PENDING ready or waiting for retry.",
			nil, nil,
		),
		outboxOldestDesc: prometheus.NewDesc(
			"sip_happens_outbox_oldest_pending_seconds",
			"Age in seconds of the oldest PENDING outbox event.",
			nil, nil,
		),
		ordersPendingDesc: prometheus.NewDesc(
			"sip_happens_orders_pending_payment_total",
			"Orders in PENDING_PAYMENT status.",
			nil, nil,
		),
		ordersOldestPendingDesc: prometheus.NewDesc(
			"sip_happens_orders_oldest_pending_payment_seconds",
			"Age in seconds of the oldest PENDING_PAYMENT order.",
			nil, nil,
		),
		ordersPastExpiresDesc: prometheus.NewDesc(
			"sip_happens_orders_pending_payment_past_expires_total",
			"PENDING_PAYMENT orders whose expires_at is in the past (reaper should cancel; non-zero is a backlog signal).",
			nil, nil,
		),
		ordersPastExpiresOldestDesc: prometheus.NewDesc(
			"sip_happens_orders_pending_payment_past_expires_oldest_seconds",
			"Seconds since expires_at of the oldest PENDING_PAYMENT order already past expires_at (reaper lag).",
			nil, nil,
		),
		invNegativeCountersDesc: prometheus.NewDesc(
			"sip_happens_inventory_negative_counter_products",
			"Products where on_hand < 0, reserved < 0, or available (on_hand - reserved) < 0.",
			nil, nil,
		),
		invReservedMismatchDesc: prometheus.NewDesc(
			"sip_happens_inventory_reserved_mismatch_products",
			"Products where inventory.reserved does not equal the sum of ACTIVE reservation row qty.",
			nil, nil,
		),
		invReservationLeaksDesc: prometheus.NewDesc(
			"sip_happens_inventory_reservation_leaks",
			"ACTIVE inventory_reservations on CANCELLED orders or PENDING_PAYMENT orders past expires_at.",
			nil, nil,
		),
	}
}

func RegisterDBStatsCollector(db *sql.DB) {
	if db == nil {
		return
	}
	prometheus.MustRegister(NewDBStatsCollector(db))
}

func (c *DBStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.outboxPendingDesc
	ch <- c.outboxOldestDesc
	ch <- c.ordersPendingDesc
	ch <- c.ordersOldestPendingDesc
	ch <- c.ordersPastExpiresDesc
	ch <- c.ordersPastExpiresOldestDesc
	ch <- c.invNegativeCountersDesc
	ch <- c.invReservedMismatchDesc
	ch <- c.invReservationLeaksDesc
}

func (c *DBStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if pending, oldest, err := c.queryOutbox(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(c.outboxPendingDesc, prometheus.GaugeValue, float64(pending))
		ch <- prometheus.MustNewConstMetric(c.outboxOldestDesc, prometheus.GaugeValue, oldest)
	}

	if ordPending, ordOldest, err := c.queryOrdersPending(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(c.ordersPendingDesc, prometheus.GaugeValue, float64(ordPending))
		ch <- prometheus.MustNewConstMetric(c.ordersOldestPendingDesc, prometheus.GaugeValue, ordOldest)
	}

	if past, pastOldest, err := c.queryOrdersPastExpires(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(c.ordersPastExpiresDesc, prometheus.GaugeValue, float64(past))
		ch <- prometheus.MustNewConstMetric(c.ordersPastExpiresOldestDesc, prometheus.GaugeValue, pastOldest)
	}

	if neg, err := c.queryInventoryNegativeCounters(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(c.invNegativeCountersDesc, prometheus.GaugeValue, float64(neg))
	}
	if mm, err := c.queryInventoryReservedMismatch(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(c.invReservedMismatchDesc, prometheus.GaugeValue, float64(mm))
	}
	if leaks, err := c.queryInventoryReservationLeaks(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(c.invReservationLeaksDesc, prometheus.GaugeValue, float64(leaks))
	}
}

func (c *DBStatsCollector) queryOutbox(ctx context.Context) (count int64, oldestSeconds float64, err error) {
	err = c.db.QueryRowContext(ctx, `
select count(*)::bigint,
       coalesce(extract(epoch from (now() - min(occurred_at))), 0)::float8
from outbox_events
where status = 'PENDING'
`).Scan(&count, &oldestSeconds)
	return
}

func (c *DBStatsCollector) queryOrdersPending(ctx context.Context) (count int64, oldestSeconds float64, err error) {
	err = c.db.QueryRowContext(ctx, `
select count(*)::bigint,
       coalesce(extract(epoch from (now() - min(created_at))), 0)::float8
from orders
where status = 'PENDING_PAYMENT'
`).Scan(&count, &oldestSeconds)
	return
}

// queryOrdersPastExpires implements roadmap 143: orders still PENDING_PAYMENT
// after their checkout reservation TTL (expires_at < now()). The cancel-expired
// job should drive these to CANCELLED; a sustained non-zero count means the
// reaper is behind or failing.
func (c *DBStatsCollector) queryOrdersPastExpires(ctx context.Context) (count int64, oldestPastExpiresSeconds float64, err error) {
	err = c.db.QueryRowContext(ctx, `
select count(*)::bigint,
       coalesce(extract(epoch from (now() - min(expires_at))), 0)::float8
from orders
where status = 'PENDING_PAYMENT'
  and expires_at is not null
  and expires_at < now()
`).Scan(&count, &oldestPastExpiresSeconds)
	return
}

func (c *DBStatsCollector) queryInventoryNegativeCounters(ctx context.Context) (count int64, err error) {
	err = c.db.QueryRowContext(ctx, `
select count(*)::bigint
from inventory
where on_hand < 0
   or reserved < 0
   or (on_hand - reserved) < 0
`).Scan(&count)
	return
}

func (c *DBStatsCollector) queryInventoryReservedMismatch(ctx context.Context) (count int64, err error) {
	err = c.db.QueryRowContext(ctx, `
with desired as (
  select product_id, coalesce(sum(qty), 0)::int as desired
  from inventory_reservations
  where status = 'ACTIVE'
  group by product_id
)
select count(*)::bigint
from inventory i
left join desired d on d.product_id = i.product_id
where i.reserved <> coalesce(d.desired, 0)
`).Scan(&count)
	return
}

func (c *DBStatsCollector) queryInventoryReservationLeaks(ctx context.Context) (count int64, err error) {
	err = c.db.QueryRowContext(ctx, `
select count(*)::bigint
from inventory_reservations r
join orders o on o.id = r.order_id
where r.status = 'ACTIVE'
  and (
    o.status = 'CANCELLED'
    or (o.status = 'PENDING_PAYMENT' and o.expires_at is not null and o.expires_at < now())
  )
`).Scan(&count)
	return
}
