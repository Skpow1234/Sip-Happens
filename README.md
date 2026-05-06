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
- **`STRIPE_MOCK=1`**: enables a local mock Stripe checkout URL when `STRIPE_SECRET_KEY` is not set (prototype only)
- **`ADMIN_TOKEN`**: bearer token to access admin endpoints (prototype only)
- **`CORE_API_HMAC_SECRET`**: shared secret used to sign requests from `ssh-gateway` to `core-api` (enables auth + replay protection for `/checkout` and `/orders/*`)

Database config is currently provided by Compose:

- `POSTGRES_DSN` is set for the `core-api` container to reach the `postgres` service.

## Current endpoints

- `GET /healthz`
- `POST /checkout` (creates a pending order + reserves inventory + creates Stripe Checkout Session when `STRIPE_SECRET_KEY` is set)
- `GET /orders/{id}`
- `POST /webhooks/stripe` (enabled when `STRIPE_WEBHOOK_SECRET` is set)
- `GET /admin/products` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `POST /admin/products` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `POST /admin/prices` (requires `Authorization: Bearer $ADMIN_TOKEN`)
- `POST /admin/inventory` (requires `Authorization: Bearer $ADMIN_TOKEN`)
