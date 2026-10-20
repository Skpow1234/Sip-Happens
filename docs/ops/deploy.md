# Deploy — make the shop reachable over SSH

Goal: customers run `ssh shop.example.com` (or `ssh -p 2222 shop.example.com`) and land in the Sip-Happens TUI. Payments stay on Stripe-hosted Checkout (browser / phone); cards never enter the SSH session.

There are **no GitHub project milestones** on this repo today. Product sequencing lives in [milestones.md](../milestones.md).

## Architecture (what must be public)

| Surface | Public? | Notes |
|---------|---------|--------|
| **ssh-gateway `:2222`** | **Yes** | Customer shop. Publish host port (Compose prod default: `SIP_HAPPENS_SSH_PORT`, often `2222` or `22`). |
| **Caddy → core-api HTTPS** | Yes (ops + Stripe) | Webhooks, admin, OpenAPI. Do **not** expose Postgres/Redis/NATS. |
| Postgres / Redis / NATS / MinIO | No | Internal Docker network only in prod overlay |

```
Internet
   │
   ├─ TCP 22/2222 ──► ssh-gateway ──HMAC──► core-api
   │
   └─ TCP 443 ──────► Caddy ──────────────► core-api
                         ▲
              Stripe webhooks / admin
```

## Option A — Docker Compose on a VM (supported path)

Fits the repo’s Docker-only ops model. Use any Linux VPS (Hetzner, DigitalOcean, AWS EC2, GCP, etc.) with Docker Engine + Compose plugin.

### 1. Prepare the host

- Open firewall: **SSH shop port** (`2222` or `22`), **443** (and **80** if Caddy does ACME HTTP-01).
- Do **not** publish `5432` / `6379` / `4222` to the internet.
- Clone the repo (or deploy a release artifact that includes Compose files + built images).

### 2. Secrets

```bash
cp infra/env.prod.example .env.prod
# Edit: POSTGRES_PASSWORD, ADMIN_TOKEN(S), CORE_API_HMAC_KEYS,
#       STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET, SIP_HAPPENS_SSH_PORT, …
```

Set `STRIPE_MOCK=0`, `NAMESPACE=prod`, `OUTBOX_PUBLISHER=nats`. Never set `SSH_DEV_FINGERPRINT` in prod.

### 3. Authorized customer keys

See [ssh-production.md](ssh-production.md). Minimal flow:

```bash
# On the host (example): create keys file, then inject into the volume after first up
cp infra/authorized_keys.example /secure/authorized_keys
# append real ed25519 public keys (one per customer / operator)
```

### 4. Bring up production

```bash
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod up -d
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod run --rm core-migrate up
```

`compose.prod.yaml` publishes **ssh-gateway** as `${SIP_HAPPENS_SSH_PORT:-2222}:2222`, clears published **core-api** ports (use Caddy), enables `SSH_REQUIRE_AUTHORIZED_KEYS=1`, and sets `restart: unless-stopped` on app services.

### 5. Point DNS + Stripe

- `A`/`AAAA` for `shop.example.com` → VM.
- Customers: `ssh -p 2222 shop.example.com` (omit `-p` if you published `22`).
- Stripe Dashboard: webhook endpoint `https://api.example.com/webhooks/stripe` (Caddy), signing secret → `STRIPE_WEBHOOK_SECRET`.
- Publish privacy / data-processing pages at the URLs in `.env.prod`.

### 6. Smoke

```bash
curl -fsS https://api.example.com/healthz
ssh -p 2222 -i ~/.ssh/sip-customer shop.example.com
```

## Option B — Staging beside prod

```bash
COMPOSE_PROJECT_NAME=sip-happens-staging \
  docker compose -f infra/compose.yaml -f infra/compose.staging.yaml --env-file .env.staging up -d
```

Use a different `NAMESPACE=staging` and host ports (`SIP_HAPPENS_SSH_PORT`, HTTP) if sharing one machine with dev. Details: [compose-environments.md](compose-environments.md).

## Option C — Orchestrators (not packaged here)

Same containers (`core-api`, `ssh-gateway`, jobs) can run on Kubernetes, Nomad, or Fly.io / Railway-style hosts:

- Expose a **TCP Service** for ssh-gateway (not HTTP ingress alone — SSH is raw TCP).
- Keep HMAC keys and Stripe secrets in the platform secret store.
- Run migrations as a Job before rolling the API.
- Prefer one stable SSH host key (Secret / volume) so clients do not thrash `known_hosts`.

You will need to supply your own manifests; this repo ships Compose overlays only.

## Go-live checklist

- [ ] `SSH_REQUIRE_AUTHORIZED_KEYS=1`, no `SSH_DEV_FINGERPRINT`
- [ ] `CORE_API_HMAC_KEYS` set (signed routes fail-closed with secrets present)
- [ ] Strong `ADMIN_TOKENS` / role splits; rotate runbooks bookmarked
- [ ] Real Stripe keys + webhook secret; `STRIPE_MOCK=0`
- [ ] Migrations applied; seed **not** used in prod (or only catalog seed without demo fingerprint)
- [ ] Workers up: `order-reaper`, `stripe-reconcile`, `outbox-dispatch`, `nats-consume`, `fulfillment-runner`
- [ ] Caddy (or other TLS) terminating HTTPS for webhooks/admin
- [ ] Firewall: shop SSH + 443 only
- [ ] Legal URLs live; age gate + INVIMA fields verified in TUI
- [ ] `/healthz` and `/readyz` green; scrape `/metrics` if you run Prometheus

## Related

- [ssh-production.md](ssh-production.md) — host key + `authorized_keys`
- [environment.md](../environment.md) — full env reference
- [local-development.md](../local-development.md) — `dev-up` path
