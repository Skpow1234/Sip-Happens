# Sip-Happens

Terminal-first beer shop for Colombia.

Users connect via **SSH** to a TUI, browse a beer catalog, build a cart, and start checkout. **Payments happen off-terminal** via **Stripe-hosted checkout** (no card entry in SSH). The backend owns order state, inventory reservations, auditability, and compliance constraints (age gate, INVIMA visibility, etc.).

## What’s in this repo (today)

- **`core-api`**: Go service with PostgreSQL persistence, basic HTTP endpoints, migrations, and Stripe webhook verification.
- **Docker-only workflow**: the host machine is assumed to have **only Docker** installed.

## Run (Docker only)

From the repo root:

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

### Start core-api

```bash
docker compose -f infra/compose.yaml up -d core-api
```

### Smoke test

```bash
curl -fsS http://localhost:8080/healthz
```

Expected:

```json
{"ok":true}
```

## Environment variables

Set these in your shell (or in your Docker environment) before starting `core-api`:

- **`STRIPE_SECRET_KEY`**: required to create Stripe Checkout Sessions from `POST /checkout`
- **`STRIPE_WEBHOOK_SECRET`**: required to verify incoming `POST /webhooks/stripe`
- **`STRIPE_MOCK=1`**: enables a local mock Stripe checkout URL when `STRIPE_SECRET_KEY` is not set (prototype only)

Database config is currently provided by Compose:

- `POSTGRES_DSN` is set for the `core-api` container to reach the `postgres` service.

## Current endpoints

- `GET /healthz`
- `POST /checkout` (creates a pending order + reserves inventory + creates Stripe Checkout Session when `STRIPE_SECRET_KEY` is set)
- `GET /orders/{id}`
- `POST /webhooks/stripe` (enabled when `STRIPE_WEBHOOK_SECRET` is set)
