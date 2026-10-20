# Observability

- **Request IDs**: `ssh-gateway` sends `X-Request-Id`; `core-api` echoes it.
- **Metrics**: scrape `GET /metrics` (HTTP, outbox backlog, pending orders, NATS consumer, inventory anomalies).
- **Readiness**: `GET /readyz` checks DB; when `OUTBOX_PUBLISHER=nats`, also pings NATS.
- **Logs**: JSON lines with `request_id`, `customer_id`, `order_id` when available.
- **Stripe DLQ**: `GET /admin/stripe/dead-letters`, replay via `POST .../replay`.
- **Compose healthchecks**: `core-api` → `/healthz`; `ssh-gateway` → TCP/`healthcheck` subcommand.

## Dashboards / alerts

Place Grafana / Prometheus assets here when published:

- `docs/observability/grafana/sip-happens.json`
- `docs/observability/prometheus/alerts.yml`

Until those files exist, scrape `/metrics` directly or import rules from your ops repo.
