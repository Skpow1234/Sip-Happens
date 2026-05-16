package observability

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DBStatsCollector exposes backlog gauges from Postgres on each scrape.
type DBStatsCollector struct {
	db *sql.DB

	outboxPendingDesc       *prometheus.Desc
	outboxOldestDesc        *prometheus.Desc
	ordersPendingDesc       *prometheus.Desc
	ordersOldestPendingDesc *prometheus.Desc
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
}

func (c *DBStatsCollector) Collect(ch chan<- prometheus.Metric) {
	if c.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pending, oldest, err := c.queryOutbox(ctx)
	if err == nil {
		ch <- prometheus.MustNewConstMetric(c.outboxPendingDesc, prometheus.GaugeValue, float64(pending))
		ch <- prometheus.MustNewConstMetric(c.outboxOldestDesc, prometheus.GaugeValue, oldest)
	}

	ordPending, ordOldest, err := c.queryOrdersPending(ctx)
	if err == nil {
		ch <- prometheus.MustNewConstMetric(c.ordersPendingDesc, prometheus.GaugeValue, float64(ordPending))
		ch <- prometheus.MustNewConstMetric(c.ordersOldestPendingDesc, prometheus.GaugeValue, ordOldest)
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
