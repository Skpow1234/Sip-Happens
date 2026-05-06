# Sip-Happens

Terminal-first beer shop for Colombia.

Users connect via **SSH** to a TUI, browse a beer catalog, build a cart, and start checkout. **Payments happen off-terminal** via **Stripe-hosted checkout** (no card entry in SSH). The backend owns order state, inventory reservations, auditability, and compliance constraints (age gate, INVIMA visibility, etc.).

## What’s in this repo (today)

- **`core-api`**: Go service with PostgreSQL persistence, basic HTTP endpoints, migrations, and Stripe webhook verification.
- **Docker-only workflow**: the host machine is assumed to have **only Docker** installed.

## Run (Docker only)

From the repo root:

### One-command dev bring-up (recommended)

```bash
bash scripts/dev-up.sh
```

### Build

```bash
docker compose -f infra/compose.yaml build
```

### Start Postgres

```bash
docker compose -f infra/compose.yaml up -d postgres
```

### Run migrations

```bash
docker compose -f infra/compose.yaml run --rm core-api migrate up
```

### Run the Stripe reconcile job (optional)

This worker re-checks stale `PENDING_PAYMENT` orders against Stripe (by `stripe_payment_intent_id`) and applies `MarkPaid` when Stripe says the payment succeeded.

- Run once:

```bash
docker compose -f infra/compose.yaml run --rm -e JOB_RUN_ONCE=true core-api jobs stripe-reconcile
```

- Run continuously:

```bash
docker compose -f infra/compose.yaml up -d stripe-reconcile
```

### Start core-api

```bash
docker compose -f infra/compose.yaml up -d core-api
```

### Start SSH gateway (TUI)

```bash
docker compose -f infra/compose.yaml up -d ssh-gateway
ssh -p 2222 localhost
```

### Start the “order reaper” (expires pending orders)

This background worker cancels expired `PENDING_PAYMENT` orders and releases inventory reservations.

```bash
docker compose -f infra/compose.yaml up -d order-reaper
```

### Start the outbox dispatcher (optional)

This background worker claims rows from `outbox_events` and publishes them (currently to logs, until we wire a real queue).

```bash
docker compose -f infra/compose.yaml run --rm core-api jobs outbox-dispatch
```

Environment knobs:

- `OUTBOX_MAX_ATTEMPTS` (default `10`)
- `OUTBOX_BASE_BACKOFF_SECONDS` (default `30`)
- `OUTBOX_LOCKED_BY` (default `outbox-dispatcher`)

### Replay outbox dead letters (optional)

If some events reach terminal `FAILED`, they are copied into `outbox_dead_letters`.
You can re-queue a batch back into `outbox_events` with:

```bash
docker compose -f infra/compose.yaml run --rm core-api jobs outbox-replay
```

Environment knobs:

- `OUTBOX_REPLAY_LIMIT` (default `100`)

### Smoke test

```bash
curl -fsS http://localhost:8080/healthz
```

Expected:

```json
{"ok":true}
```

## Stop / clean

- Stop containers (keep DB volume/data):

```bash
bash scripts/dev-down.sh
```

- Full reset (removes volumes, deletes local DB data):

```bash
bash scripts/dev-reset.sh
```

## Environment variables

Set these in your shell (or in your Docker environment) before starting `core-api`:

- **`STRIPE_SECRET_KEY`**: required to create Stripe Checkout Sessions from `POST /checkout`
- **`STRIPE_WEBHOOK_SECRET`**: required to verify incoming `POST /webhooks/stripe`
- **`STRIPE_WEBHOOK_PATH`**: optional path to mount Stripe webhook handler (defaults to `/webhooks/stripe`)
- **`STRIPE_MOCK=1`**: enables a local mock Stripe checkout URL when `STRIPE_SECRET_KEY` is not set (prototype only)
- **`ADMIN_TOKENS`**: comma-separated bearer tokens to access admin endpoints (supports rotation)
- **`ADMIN_TOKEN`**: legacy single admin token (prefer `ADMIN_TOKENS`)
- **`CORE_API_HMAC_SECRET`**: shared secret used to sign requests from `ssh-gateway` to `core-api` (enables auth + replay protection for `/checkout` and `/orders/*`)

Database config is currently provided by Compose:

- `POSTGRES_DSN` is set for the `core-api` container to reach the `postgres` service.

## Current endpoints

- `GET /healthz`
- `GET /readyz` (DB readiness; returns 503 until DB is reachable)
- `GET /metrics` (Prometheus metrics)
- `POST /checkout` (creates a pending order + reserves inventory + creates Stripe Checkout Session when `STRIPE_SECRET_KEY` is set)
- `GET /orders/{id}`
- `POST /webhooks/stripe` (enabled when `STRIPE_WEBHOOK_SECRET` is set)
- `GET /admin/products` (requires `Authorization: Bearer $ADMIN_TOKEN`; supports `limit`, `offset`, `q`, `active`, `currency`)
- `POST /admin/products` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `PUT /admin/products/{id}` (requires `Authorization: Bearer $ADMIN_TOKEN`; full replace)
- `PATCH /admin/products/{id}` (requires `Authorization: Bearer $ADMIN_TOKEN`; partial update)
- `DELETE /admin/products/{id}` (requires `Authorization: Bearer $ADMIN_TOKEN`; deactivates product)
- `POST /admin/prices` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `PUT /admin/prices` (requires `Authorization: Bearer $ADMIN_TOKEN`; idempotent-ish alias of POST for tooling)
- `DELETE /admin/prices?product_id=...&currency=COP` (requires `Authorization: Bearer $ADMIN_TOKEN`; ends current price)
- `POST /admin/inventory` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `PUT /admin/inventory` (requires `Authorization: Bearer $ADMIN_TOKEN`; idempotent upsert)
- `GET /admin/audit-events` (requires `Authorization: Bearer $ADMIN_TOKEN`; supports `limit`, `offset`, `aggregate_type`, `aggregate_id`)

## Observability

- **Request IDs**: `ssh-gateway` sends `X-Request-Id` to `core-api` and `core-api` echoes it back on responses.
- **Metrics**: scrape `GET /metrics`
- **Readiness**: `GET /readyz` checks DB connectivity

## Error responses

All API errors return JSON in a stable envelope:

```json
{
  "error": {
    "code": "invalid_input",
    "message": "request invalid",
    "request_id": "..."
  }
}
```

## Tests

Unit tests run without any dependencies.

Integration/contract tests require a Postgres DSN available to the `go test` process:

- **Run all tests** (skips DB-backed tests if `POSTGRES_DSN` is missing):

```bash
go test ./...
```

- **Run DB-backed tests** (example DSN, adjust as needed):

```bash
POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" go test ./...
```
