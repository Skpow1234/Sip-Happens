# Sip-Happens

Terminal-first beer shop for Colombia.

Users connect via **SSH** to a TUI, browse a beer catalog, build a cart, and start checkout. **Payments happen off-terminal** via **Stripe-hosted checkout** (no card entry in SSH). The backend owns order state, inventory reservations, auditability, and compliance constraints (age gate, INVIMA visibility, etc.).

## What’s in this repo (today)

- **`core-api`**: Go service with PostgreSQL persistence, HTTP endpoints for catalog / cart / orders / subscriptions / addresses / payment methods / admin, migrations, and Stripe webhook verification. Aiming at [Terminal.shop API parity](docs/adr/0020-terminal-shop-api-parity.md) (adapted for Colombia + SSH + HMAC, **no card data in SSH**).
- **`ssh-gateway`**: Bubble Tea TUI served over SSH (split-pane catalog, account area with pedidos / suscripciones / direcciones, cart breadcrumbs, off-terminal Stripe payment).
- **Docker-only workflow**: the host machine is assumed to have **only Docker** installed.

## Run (Docker only)

Base images are pinned by digest; CVE remediation: [`docs/ops/docker-image-security.md`](docs/ops/docker-image-security.md).

From the repo root:

### One-command dev bring-up (recommended)

```bash
bash scripts/dev-up.sh
```

Uses `infra/compose.yaml` plus `infra/compose.dev.yaml` (publishes Postgres, Redis, NATS, MinIO, and API ports on localhost). For a isolated **staging** stack (`NAMESPACE=staging`), see [`docs/ops/compose-environments.md`](docs/ops/compose-environments.md) and `./scripts/stack-up.sh staging`.

### Production Compose profile

Base `infra/compose.yaml` keeps databases on the internal Docker network only. For production:

```bash
cp infra/env.prod.example .env.prod
# edit secrets, then:
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod up -d
```

`compose.prod.yaml` loads secrets via `env_file`, sets `restart: unless-stopped`, disables published `core-api` port (use Caddy), and defaults `SSH_REQUIRE_AUTHORIZED_KEYS=1`.

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
docker compose -f infra/compose.yaml -f infra/compose.dev.yaml build
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

`bash scripts/dev-up.sh` already starts `ssh-gateway` and seeds a demo customer linked to the dev SSH key.

There is **no password** — auth is public-key only. From the repo root:

```bash
ssh -F infra/demo/ssh_config sip-happens
```

Uses a **fixed dev host key** and project `infra/demo/known_hosts`, so container recreates do not trip `REMOTE HOST IDENTIFICATION HAS CHANGED`. See [`infra/demo/README.md`](infra/demo/README.md).

If `infra/demo/sip-happens-demo` is missing (private key is gitignored), generate it once:

```bash
ssh-keygen -t ed25519 -f infra/demo/sip-happens-demo -N "" -C sip-happens-demo@local
```

The matching public key is in `infra/demo/authorized_keys` (mounted into the gateway). See [`infra/demo/README.md`](infra/demo/README.md).

Flow: confirm age → split-pane catalog → cart → delivery (breadcrumb steps) → Stripe checkout URL off-terminal (mock when `STRIPE_MOCK=1`).

TUI shortcuts: `s` shop · `a` account · `c` cart · `?` help · `←`/`→` add/remove on catalog · `b` suscribirse (recurring subscription on selected product) · `enter` checkout from cart.

Account sections (`a` then `↑/↓` or `tab`): pedidos · suscripciones · direcciones guardadas · tokens de API (`n` nuevo, `x` revocar) · preguntas frecuentes · acerca de.

Dev HMAC between gateway and API is pre-wired in Compose (`CORE_API_HMAC_KEY_ID=dev`, `CORE_API_HMAC_KEYS=dev:dev-hmac-secret`).

**Production SSH**: set `SSH_REQUIRE_AUTHORIZED_KEYS=1` and mount `authorized_keys` at `SSH_AUTHORIZED_KEYS_PATH` (see `infra/authorized_keys.example`). Each SSH public key maps to a stable `SHA256:…` customer fingerprint (OpenSSH-compatible, same as `ssh-keygen -lf`).

**Host key**: local dev mounts a committed key at `infra/demo/ssh_host_ed25519_key` (see `compose.dev.yaml`); production uses volume `sshkeys` — [`docs/ops/ssh-production.md`](docs/ops/ssh-production.md).

Catalog TUI shortcuts: `/` search, `f` filters (style, brewery, ABV), `n`/`p` pages.

### Start the “order reaper” (expires pending orders)

This background worker cancels expired `PENDING_PAYMENT` orders and releases inventory reservations.

```bash
docker compose -f infra/compose.yaml up -d order-reaper
```

### Start the outbox dispatcher (optional)

This background worker claims rows from `outbox_events` and publishes them using **`OUTBOX_PUBLISHER`**:

| Value | Behavior |
|-------|----------|
| `nats` (Compose default) | Publish to NATS JetStream (`NATS_URL`, `NATS_STREAM` required). `nats-consume` can process `order.paid`. |
| `log` (or unset) | Structured log lines only (`outbox_publish`); no NATS connection. Use for local debugging without a message bus. |

```bash
docker compose -f infra/compose.yaml -f infra/compose.dev.yaml up -d outbox-dispatch
```

Log-only example:

```bash
OUTBOX_PUBLISHER=log docker compose -f infra/compose.yaml -f infra/compose.dev.yaml up -d outbox-dispatch
```

Outbox env knobs: `OUTBOX_MAX_ATTEMPTS`, `OUTBOX_BASE_BACKOFF_SECONDS`, `OUTBOX_LOCKED_BY`, `NAMESPACE`, `NATS_SUBJECT_PREFIX`.

**JetStream subjects (ADR 0016)** — set `NAMESPACE` (default `dev`) on `outbox-dispatch` and `nats-consume` so publishers and consumers agree:

| Kind | Subject pattern |
|------|-----------------|
| Events | `sip_happens.{NAMESPACE}.events.{event_type}` |
| Consumer filter | `sip_happens.{NAMESPACE}.events.>` |
| DLQ | `sip_happens.{NAMESPACE}.dlq` |

Optional overrides (`NATS_SUBJECT_PREFIX`, `NATS_CONSUME_SUBJECT`, `NATS_DLQ_SUBJECT`) must still match the same `NAMESPACE`.

### NATS consumers (fulfillment handoff)

`bash scripts/dev-up.sh` also starts `nats-consume` (handles `orders.order.paid` → shipment) and `fulfillment-runner`.

Ops: [`docs/ops/nats-consumer-playbook.md`](docs/ops/nats-consumer-playbook.md)

### Inventory reconcile job (optional)

Background worker that **detects** inventory drift (ACTIVE reservations on cancelled/expired orders; `inventory.reserved` not matching the sum of ACTIVE reservation rows). By default it only logs mismatches; set `INV_RECONCILE_FIX=1` to apply fixes.

```bash
docker compose -f infra/compose.yaml run --rm core-jobs inventory-reconcile
```

Environment knobs:

- `JOB_INTERVAL_SECONDS` (default `300`)
- `JOB_BATCH_LIMIT` (default `500`)
- `JOB_RUN_ONCE` — exit after one pass (`true`/`1`)
- `INV_RECONCILE_FIX` — apply fixes when `true`/`1` (default off)
- `INV_EXPIRE_GRACE_SECONDS` — grace after `expires_at` before treating pending orders as expired leaks (default `300`)

### Fulfillment runner job

Polls **PAID** orders and advances the fulfillment saga (creates shipment, moves order to **FULFILLING**). Compose service `fulfillment-runner` runs this continuously in dev.

```bash
docker compose -f infra/compose.yaml up -d fulfillment-runner
# one-shot:
docker compose -f infra/compose.yaml run --rm core-jobs fulfillment-runner
```

Uses the same `JOB_INTERVAL_SECONDS`, `JOB_BATCH_LIMIT`, and `JOB_RUN_ONCE` knobs as other `core-jobs` workers.

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

**Secret handling**: Stripe keys, webhook secrets, and bearer/HMAC material are redacted in structured logs (`pkg/redact`) and in `audit_events` / payment / Stripe event storage. Rotation runbooks: [`docs/runbooks/rotate-admin-tokens.md`](docs/runbooks/rotate-admin-tokens.md), [`docs/runbooks/rotate-core-api-hmac-keys.md`](docs/runbooks/rotate-core-api-hmac-keys.md).
- **`SSH_REQUIRE_AUTHORIZED_KEYS`**: set to `1` in production to require `authorized_keys`
- **`SSH_AUTHORIZED_KEYS_PATH`**: path inside ssh-gateway container (default `/data/authorized_keys`)
- **`SSH_DEV_FINGERPRINT`**: dev-only fallback fingerprint when no SSH public key is offered (Compose default matches `infra/demo/sip-happens-demo.pub` and `seed demo`)
- **`PRIVACY_POLICY_URL`**: shown on ssh-gateway TUI, emails, and admin console (default `https://sip-happens.local/privacy`)
- **`CONSUMER_RIGHTS_URL`**: SIC consumer-protection link on TUI, emails, and admin console (default `https://www.sic.gov.co/proteccion-del-consumidor`)
- **`DATA_PROCESSING_POLICY_URL`**: Ley 1581 data-processing notice linked at TUI checkout confirm and in email footers (default `https://sip-happens.local/legal/data-processing`; source copy in [`docs/legal/data-processing.md`](docs/legal/data-processing.md))
- **`SSH_DISABLE_PASSWORD_AUTH`**: reject password SSH logins (default `1` in Compose)
- **`SSH_DISABLE_PORT_FORWARDING`**: deny SSH port forwarding (default `1`)
- **`FEATURE_FLAGS`**: comma-separated toggles — see [Feature flags](#feature-flags) below
- **`OUTBOX_PUBLISHER`**: `nats` or `log` — selects outbox-dispatch publisher (see [outbox dispatcher](#start-the-outbox-dispatcher-optional))
- **`REDIS_URL`**: optional Redis URL (Compose sets `redis://redis:6379/0` for `core-api`)
- **`REDIS_REQUIRED`**: set to `1` to fail `/readyz` when Redis is required but unavailable
- **`EMAIL_PROVIDER`**: `log` (default; prints to stdout) or `smtp`
- **`EMAIL_FROM`**: sender address (default `noreply@sip-happens.local`)
- **`SMTP_HOST`**, **`SMTP_PORT`** (default `587`), **`SMTP_USER`**, **`SMTP_PASSWORD`**: required when `EMAIL_PROVIDER=smtp`

Transactional email (Spanish templates) is sent only when the matching flag is set and the customer has an email on file: payment link after `POST /checkout` (`email_payment_link`), receipt when `order.paid` is consumed (`email_receipt` on `nats-consume`).

### Feature flags

Set on **core-api** and **nats-consume** (email receipt) via `FEATURE_FLAGS` (comma-separated, case-insensitive). Canonical list: `pkg/featureflags/known.go`.

| Flag | Service | Effect |
|------|---------|--------|
| `catalog_cache` | core-api | In-memory SWR cache for catalog GET routes (per replica). |
| `catalog_cache_redis` | core-api | Shared catalog cache in Redis; requires `REDIS_URL`. |
| `redis_rate_limit` | core-api | Shared checkout/admin rate limits in Redis; requires `REDIS_URL`. See [`docs/ops/rate-limit-scaling.md`](docs/ops/rate-limit-scaling.md). |
| `email_payment_link` | core-api | Email Stripe checkout URL after `POST /checkout`. |
| `email_receipt` | nats-consume | Email receipt when `order.paid` is handled; needs `OUTBOX_PUBLISHER=nats`. |

Example (typical Compose dev):

```bash
FEATURE_FLAGS=catalog_cache_redis,redis_rate_limit
```

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

Full contract: [`docs/openapi.yaml`](docs/openapi.yaml) (also served at `GET /openapi.yaml`; Redoc at `GET /docs` with admin Bearer token). Terminal.shop parity rationale: [`docs/adr/0020-terminal-shop-api-parity.md`](docs/adr/0020-terminal-shop-api-parity.md).

Public ids in JSON use stable prefixes: `prd_…` (product), `addr_…` (address), `pm_…` (Stripe payment method), `sub_…` (subscription). Orders also expose a human-readable `display_ref` (`SH-…`) accepted on `/orders/{id}`.

**Public**

- `GET /healthz`, `GET /readyz` (DB readiness; 503 until DB is reachable), `GET /metrics`
- `GET /catalog?currency=COP` — list active products with price and `available_qty` (`limit`, `offset`, `q`, `style`, `brewery`, `abv_min`, `abv_max`)
- `GET /catalog/{id}?currency=COP` — product detail (`prd_…` public id, UUID, or SKU) with `available_qty`
- `GET /openapi.yaml` — OpenAPI 3 spec

**Signed — profile** (HMAC v2 + `X-SSH-Fingerprint`; used by `ssh-gateway`)

- `GET /customers/me`, `PATCH /customers/me` — profile (snake_case JSON)
- `GET /profile`, `PUT /profile`, `PATCH /profile` — Terminal.shop aliases (`profile` wrapper)

**Signed — addresses**

- `GET /addresses`, `POST /addresses`, `GET /addresses/{id}`, `DELETE /addresses/{id}` — saved delivery addresses (Bogotá, Medellín, Cali)

**Signed — payment methods** (Stripe-hosted; **no card data ever crosses SSH**)

- `POST /payment-methods/collect` — start a hosted Stripe Setup/Checkout Session to save a card; returns `{ collect: { url, checkout_session_id } }`
- `GET /payment-methods` — list saved cards on the customer’s Stripe customer
- `GET /payment-methods/{id}` — get one (`pm_…`), 404 when not owned
- `DELETE /payment-methods/{id}` — detach saved card

**Signed — cart**

- `GET /cart` — server-persistent cart with live prices, subtotal, and optional saved `address_id` / `payment_method_id`
- `PUT /cart` — bulk replace cart lines
- `DELETE /cart` — clear all lines, saved `address_id`, and `payment_method_id`
- `PUT /cart/items/{id}` — set quantity for one product (`qty: 0` removes); `id` accepts `prd_…`, UUID, or SKU
- `PUT /cart/address` — attach saved `address_id` (empty string clears)
- `PUT /cart/payment-method` — attach saved Stripe `payment_method_id` (empty string clears)
- `POST /cart/convert` — convert cart → `PENDING_PAYMENT` + Stripe Checkout URL (`Idempotency-Key` required); clears cart on success

**Signed — orders**

- `GET /orders` — list caller’s orders (`limit`, default 20, max 100; `offset`, `has_more`); no delivery PII in list view
- `POST /orders` — direct checkout from `items` + delivery (no server cart) → Stripe Checkout URL (`Idempotency-Key` required); does **not** touch the server cart
- `GET /orders/{id}` — order detail (internal UUID or `display_ref`; scoped to caller’s fingerprint)

**Signed — subscriptions** (Stripe Billing; Checkout in subscription mode)

- `GET /subscriptions` — list caller’s subscriptions (`limit`, `offset`, `has_more`)
- `POST /subscriptions` — create `PENDING` subscription + hosted Stripe subscription Checkout URL (`Idempotency-Key` required; body: `product_id`, `qty`, `interval` (`week`|`month`), optional `address_id`, `payment_method_id`, success/cancel URLs)
- `GET /subscriptions/{id}` — detail (`sub_…` or internal UUID)
- `PUT /subscriptions/{id}` — update `qty` and/or `address_id`
- `DELETE /subscriptions/{id}` — cancel (cancels Stripe subscription when linked); idempotent

**Signed — personal access tokens** (for scripts / non-SSH API access)

- `GET /tokens` — list caller’s PATs (`limit`, `offset`, `has_more`); raw secrets are **never** included
- `POST /tokens` — create a token (body: `name`, `scopes[]`, optional `expires_at`); the raw `pat_<prefix>_<secret>` is returned exactly **once** in `token`. Server stores SHA-256(raw) + indexed prefix only.
- `GET /tokens/{id}` — metadata for one PAT (`pat_…` public id or UUID)
- `DELETE /tokens/{id}` — revoke (idempotent). Available scopes: `catalog:read`, `cart:write`, `orders:read`, `orders:write`, `subscriptions:read`, `subscriptions:write`, `profile:read`, `profile:write`.

**OAuth 2.0 — third-party app registration**

- `GET /oauth/clients` — list caller’s registered apps (`limit`, `offset`, `has_more`); secrets are **never** included
- `POST /oauth/clients` — register a new app (body: `name`, `redirect_uris[]`, `scopes[]`, optional `confidential` default `true`). Returns the new `oauth_client` plus a raw `client_secret` (`ocs_<prefix>_<secret>`) **shown exactly once**. Each `redirect_uri` must be absolute; `http://` only on loopback (RFC 8252 §7.3).
- `GET /oauth/clients/{id}` — metadata for one app (`oc_…` public id or internal UUID)
- `DELETE /oauth/clients/{id}` — revoke (idempotent)

**OAuth 2.0 — Authorization Code grant**

Sip-Happens has no browser session, so the customer-side step runs through the **signed** SSH/HMAC channel:

- `POST /oauth/authorize` *(signed)* — body: `client_id` (`oci_…`), `redirect_uri`, `scopes[]`, optional `state`. Returns a single-use `oac_<prefix>_<secret>` code (10-minute TTL). The customer pastes the code into the third-party app.
- `POST /oauth/token` *(public, RFC 6749 `application/x-www-form-urlencoded`)* — body: `grant_type=authorization_code`, `code`, `redirect_uri`, plus client credentials (HTTP Basic auth or form `client_id` / `client_secret`). Returns `{ access_token (pat_…), token_type=Bearer, expires_in, scope }`. The access token is persisted in `personal_access_tokens` so a single Bearer middleware can authenticate both SSH-minted PATs and OAuth-issued tokens. Error envelope is the RFC 6749 §5.2 shape (`error`, `error_description`).

**Signed — checkout & shipping**

- `POST /checkout` — pending order, inventory reservation, Stripe Checkout Session (`Idempotency-Key` required). Legacy/explicit entry point; `POST /cart/convert` and `POST /orders` share the same engine.
- `GET /shipping/estimate?city=...` — flat shipping estimate by city

**Webhooks**

- `POST /webhooks/stripe` (when `STRIPE_WEBHOOK_SECRET` is set; path overridable via `STRIPE_WEBHOOK_PATH`)

**Admin** (`Authorization: Bearer` token; optional RBAC via `ADMIN_SUPPORT_TOKENS` / `ADMIN_FINANCE_TOKENS`; optional `X-Admin-Actor` for audit)

- `GET /admin/products` (`limit`, `offset`, `q`, `active`, `currency`)
- `POST /admin/products`, `PUT /admin/products/{id}`, `PATCH /admin/products/{id}`, `DELETE /admin/products/{id}`
- `POST /admin/prices`, `PUT /admin/prices`, `DELETE /admin/prices?product_id=...&currency=COP`
- `GET /admin/inventory` — list rows with `on_hand`, `reserved`, `available_qty` (`product_id`, `sku`, `limit`, `offset`)
- `GET /admin/inventory/reservations` — ACTIVE reservations per SKU (`product_id`, `sku`, `limit`, `offset`)
- `POST /admin/inventory`, `PUT /admin/inventory`
- `GET /admin/orders` — list orders (`status`, `created_from`, `created_to`, `limit`, `offset`)
- `GET /admin/orders/{id}` — order, items, payments, optional shipment
- `POST /admin/orders/{id}/cancel` — cancel `PENDING_PAYMENT`, release reservations (`Idempotency-Key` required)
- `POST /admin/orders/{id}/reconcile-stripe` — poll Stripe PI, mark paid if succeeded (`Idempotency-Key` required)
- `PATCH /admin/orders/{id}/shipment` — set carrier + tracking (ops); advances shipment when packed
- `GET /admin/audit-events` (`limit`, `offset`, `aggregate_type`, `aggregate_id`, `order_id` — matches `order` aggregate or JSON `order_id` in payload)
- `GET /admin/orders/{id}` includes `payments` and `payment_status_history` (normalized checkout/webhook/reconcile timeline)
- `GET /admin/orders/export` — CSV download for finance (`status`, `created_from`, `created_to`, `limit` up to 10000; same filters as list)
- `GET /docs` — API browser (Redoc)
- `GET /admin/console` — minimal read-only ops UI (orders, products, inventory, reservations); finance+ role

**Incident runbooks**

- [`docs/runbooks/incident-stripe-webhook-outage.md`](docs/runbooks/incident-stripe-webhook-outage.md)
- [`docs/runbooks/incident-postgres-failover.md`](docs/runbooks/incident-postgres-failover.md)

**Security runbooks**

- [`docs/runbooks/rotate-ssh-host-key.md`](docs/runbooks/rotate-ssh-host-key.md)
- [`docs/runbooks/rotate-admin-tokens.md`](docs/runbooks/rotate-admin-tokens.md)
- [`docs/runbooks/rotate-core-api-hmac-keys.md`](docs/runbooks/rotate-core-api-hmac-keys.md)

## Observability

- **Request IDs**: `ssh-gateway` sends `X-Request-Id` to `core-api` and `core-api` echoes it back on responses.
- **Metrics**: scrape `GET /metrics`
- **Readiness**: `GET /readyz` checks DB connectivity; when `OUTBOX_PUBLISHER=nats`, also pings NATS
- **Stripe DLQ**: `GET /admin/stripe/dead-letters`, `POST /admin/stripe/dead-letters/replay?id=` or `?limit=` (ops)
- **Compose healthchecks**: `core-api` probes `GET /healthz`; `ssh-gateway` TCP-checks the SSH listener (`/app/ssh-gateway healthcheck`)

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

## Legal and data retention

- Policy draft (Ley 1581): [`docs/legal/data-processing.md`](docs/legal/data-processing.md) — publish at `DATA_PROCESSING_POLICY_URL` in production.
- Default retention guidance: orders and compliance evidence **7 years**; customer profile **2 years** after last order; request logs **90 days** (adjust with counsel).
- Admin access to customer PII (`GET /admin/orders/{id}`, `GET /admin/orders/export`) writes `customer.pii_viewed` audit events.

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

**E2E smoke** (against a running API with demo seed):

```bash
go run ./services/core-api/cmd/api seed demo
# start core-api (Compose or HTTP_ADDR=:8080 go run ./services/core-api/cmd/api)
bash scripts/e2e-smoke.sh
```

CI runs the same checks after migrations (`go run ./services/core-api/cmd/e2e-smoke`).

**Load / concurrency** (requires Postgres):

```bash
go test ./services/core-api/internal/orders/... -run TestConcurrentCreatePendingOrder -count=1
go test ./services/core-api/internal/orders/... -bench=BenchmarkCreatePendingOrder_Concurrent -benchtime=2s -run=^$
```

**Outbox multi-node claim** (two `outbox-dispatch` workers, shared DB):

```bash
bash scripts/outbox-multi-node-ci.sh
```
