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

### Stripe webhooks (local, signed deliveries)

`STRIPE_MOCK=1` (Compose default when no secret key) skips real Checkout; to exercise **`POST /webhooks/stripe`** end-to-end, use the Stripe CLI forwarder from Compose and set **`STRIPE_WEBHOOK_SECRET`** on `core-api` to the signing secret printed by `listen`.

- Full steps: [`docs/runbooks/stripe-webhook-local.md`](docs/runbooks/stripe-webhook-local.md)
- After `dev-up`, forward events (requires `STRIPE_API_KEY` for the CLI container):

```bash
docker compose -f infra/compose.yaml run --rm stripe-cli \
  listen --forward-to http://core-api:8080/webhooks/stripe
```

Then set `STRIPE_WEBHOOK_SECRET` from the CLI output and `docker compose -f infra/compose.yaml up -d --force-recreate core-api`.

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
docker compose -f infra/compose.yaml run --rm core-migrate up
```

### Run the Stripe reconcile job (optional)

This worker re-checks stale `PENDING_PAYMENT` orders against Stripe (by `stripe_payment_intent_id`) and applies `MarkPaid` when Stripe says the payment succeeded.

- Run once:

```bash
docker compose -f infra/compose.yaml run --rm -e JOB_RUN_ONCE=true core-jobs stripe-reconcile
```

- Run continuously:

```bash
docker compose -f infra/compose.yaml up -d stripe-reconcile
```

### Start core-api

```bash
docker compose -f infra/compose.yaml up -d core-api
```

### TUI (SSH)

`bash scripts/dev-up.sh` already starts `ssh-gateway`. Connect with:

```bash
ssh -p 2222 localhost
```

Flow: confirm age → menu → browse catalog → cart → delivery form → Stripe checkout URL (mock when `STRIPE_MOCK=1`).

Dev HMAC between gateway and API is pre-wired in Compose (`CORE_API_HMAC_KEY_ID=dev`, `CORE_API_HMAC_KEYS=dev:dev-hmac-secret`).

**Production SSH**: set `SSH_REQUIRE_AUTHORIZED_KEYS=1` and mount `authorized_keys` at `SSH_AUTHORIZED_KEYS_PATH` (see `infra/authorized_keys.example`). Each SSH public key maps to a stable `SHA256:…` customer fingerprint (OpenSSH-compatible, same as `ssh-keygen -lf`).

**Host key**: persist `/data` (Compose volume `sshkeys`) so `SSH_HOST_KEY_PATH` survives restarts — see [`docs/ops/ssh-production.md`](docs/ops/ssh-production.md).

Catalog TUI shortcuts: `/` search, `f` filters (style, brewery, ABV), `n`/`p` pages.

### Start the “order reaper” (expires pending orders)

This background worker cancels expired `PENDING_PAYMENT` orders and releases inventory reservations.

```bash
docker compose -f infra/compose.yaml up -d order-reaper
```

### Start the outbox dispatcher (optional)

This background worker claims rows from `outbox_events` and publishes them to **NATS JetStream** when `OUTBOX_PUBLISHER=nats` (default in Compose).

```bash
docker compose -f infra/compose.yaml up -d outbox-dispatch
```

### NATS consumers (fulfillment handoff)

`bash scripts/dev-up.sh` also starts `nats-consume` (handles `orders.order.paid` → shipment) and `fulfillment-runner`.

Ops: [`docs/ops/nats-consumer-playbook.md`](docs/ops/nats-consumer-playbook.md)

Environment knobs:

- `OUTBOX_PUBLISHER` (`nats` or `log` for local debugging without NATS)
- `NATS_URL`, `NATS_STREAM`
- `OUTBOX_MAX_ATTEMPTS` (default `10`)
- `OUTBOX_BASE_BACKOFF_SECONDS` (default `30`)
- `OUTBOX_LOCKED_BY` (default `outbox-dispatcher`)

### Replay outbox dead letters (optional)

If some events reach terminal `FAILED`, they are copied into `outbox_dead_letters`.
You can re-queue a batch back into `outbox_events` with:

```bash
docker compose -f infra/compose.yaml run --rm core-jobs outbox-replay
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
- **`ADMIN_TOKENS`**: comma-separated bearer tokens to access admin endpoints (supports rotation); treated as **ops** (full admin) unless also listed under support/finance envs
- **`ADMIN_TOKEN`**: legacy single admin token (prefer `ADMIN_TOKENS`)
- **`ADMIN_SUPPORT_TOKENS`**: comma-separated tokens with **support** role (read admin + cancel pending orders)
- **`ADMIN_FINANCE_TOKENS`**: comma-separated tokens with **finance** role (read-only admin)
- **`CORE_API_HMAC_SECRET`**: legacy v1 shared secret for gateway → API signing
- **`CORE_API_HMAC_KEY_ID`** / **`CORE_API_HMAC_KEYS`**: v2 signing (`kid:secret` pairs; preferred)
- **`SSH_REQUIRE_AUTHORIZED_KEYS`**: set to `1` in production to require `authorized_keys`
- **`SSH_AUTHORIZED_KEYS_PATH`**: path inside ssh-gateway container (default `/data/authorized_keys`)
- **`SSH_DEV_FINGERPRINT`**: dev-only fallback fingerprint when no SSH public key is offered (Compose default matches demo seed customer)
- **`SSH_DISABLE_PASSWORD_AUTH`**: reject password SSH logins (default `1` in Compose)
- **`SSH_DISABLE_PORT_FORWARDING`**: deny SSH port forwarding (default `1`)
- **`FEATURE_FLAGS`**: comma-separated toggles (e.g. `catalog_cache`, `catalog_cache_redis`, `redis_rate_limit`, `email_payment_link`, `email_receipt`)
- **`REDIS_URL`**: optional Redis URL (Compose sets `redis://redis:6379/0` for `core-api`)
- **`REDIS_REQUIRED`**: set to `1` to fail `/readyz` when Redis is required but unavailable
- **`EMAIL_PROVIDER`**: `log` (default; prints to stdout) or `smtp`
- **`EMAIL_FROM`**: sender address (default `noreply@sip-happens.local`)
- **`SMTP_HOST`**, **`SMTP_PORT`** (default `587`), **`SMTP_USER`**, **`SMTP_PASSWORD`**: required when `EMAIL_PROVIDER=smtp`

Transactional email (Spanish templates) is sent only when the matching flag is set and the customer has an email on file: payment link after `POST /checkout` (`email_payment_link`), receipt when `order.paid` is consumed (`email_receipt` on `nats-consume`).

Object storage (MinIO, optional in Compose):

- **`MINIO_ENDPOINT`**, **`MINIO_ACCESS_KEY`**, **`MINIO_SECRET_KEY`**, **`MINIO_BUCKET`** (default `product-images`)
- **`MINIO_PUBLIC_URL`**: public base URL stored in `products.image_url` (e.g. `http://localhost:9000/product-images`)
- **`MINIO_USE_SSL`**: `1` when the endpoint uses TLS
- Upload product images: `POST /admin/products/{id}/image` (`multipart/form-data`, field `image`, ops token)

TLS reverse proxy (Caddy, optional in Compose):

- **`https://localhost:8443`** → `core-api:8080` with internal TLS (`infra/caddy/Caddyfile`)
- Use for Stripe webhook forwarding and admin API over HTTPS in local dev (`curl -k` until you trust the Caddy CA)

Observability:

- **`GET /metrics`** — Prometheus (HTTP, outbox backlog, pending orders, NATS consumer)
- Grafana dashboard: [`docs/observability/grafana/sip-happens.json`](docs/observability/grafana/sip-happens.json)
- Alert rules: [`docs/observability/prometheus/alerts.yml`](docs/observability/prometheus/alerts.yml)
- Logs are JSON lines with `request_id`, `customer_id`, and `order_id` when available

Database config is currently provided by Compose:

- `POSTGRES_DSN` is set for the `core-api` container to reach the `postgres` service.

## Current endpoints

Full contract: [`docs/openapi.yaml`](docs/openapi.yaml) (also served at `GET /openapi.yaml`; Redoc at `GET /docs` with admin Bearer token).

**Public**

- `GET /healthz`, `GET /readyz` (DB readiness; 503 until DB is reachable), `GET /metrics`
- `GET /catalog?currency=COP` — list active products with price and `available_qty` (`limit`, `offset`, `q`, `style`, `brewery`, `abv_min`, `abv_max`)
- `GET /catalog/{sku}?currency=COP` — product detail with `available_qty`
- `GET /openapi.yaml` — OpenAPI 3 spec

**Signed** (HMAC v2 + `X-SSH-Fingerprint`; used by `ssh-gateway`)

- `POST /checkout` — pending order, inventory reservation, Stripe Checkout Session (`Idempotency-Key` required)
- `GET /orders` — list caller’s orders (`limit`, default 20, max 100)
- `GET /orders/{id}` — order detail (scoped to caller’s fingerprint)
- `GET /customers/me` — profile for prefill
- `GET /cart`, `PUT /cart` — server-persistent cart
- `GET /shipping/estimate?city=...` — flat shipping estimate by city

**Webhooks**

- `POST /webhooks/stripe` (when `STRIPE_WEBHOOK_SECRET` is set; path overridable via `STRIPE_WEBHOOK_PATH`)

**Admin** (`Authorization: Bearer` token; optional RBAC via `ADMIN_SUPPORT_TOKENS` / `ADMIN_FINANCE_TOKENS`; optional `X-Admin-Actor` for audit)

- `GET /admin/products` (`limit`, `offset`, `q`, `active`, `currency`)
- `POST /admin/products`, `PUT /admin/products/{id}`, `PATCH /admin/products/{id}`, `DELETE /admin/products/{id}`
- `POST /admin/prices`, `PUT /admin/prices`, `DELETE /admin/prices?product_id=...&currency=COP`
- `POST /admin/inventory`, `PUT /admin/inventory`
- `GET /admin/orders` — list orders (`status`, `created_from`, `created_to`, `limit`, `offset`)
- `GET /admin/orders/{id}` — order, items, payments, optional shipment
- `POST /admin/orders/{id}/cancel` — cancel `PENDING_PAYMENT`, release reservations (`Idempotency-Key` required)
- `POST /admin/orders/{id}/reconcile-stripe` — poll Stripe PI, mark paid if succeeded (`Idempotency-Key` required)
- `PATCH /admin/orders/{id}/shipment` — set carrier + tracking (ops); advances shipment when packed
- `GET /admin/audit-events` (`limit`, `offset`, `aggregate_type`, `aggregate_id`)
- `GET /docs` — API browser (Redoc)

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
