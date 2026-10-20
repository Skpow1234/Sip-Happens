# Environment variables

Set these in the shell or Docker env before starting `core-api` / workers / `ssh-gateway`. Compose examples: `infra/env.dev.example`, `infra/env.staging.example`, `infra/env.prod.example`.

## Payments (Stripe)

| Variable | Purpose |
|----------|---------|
| `STRIPE_SECRET_KEY` | Create Checkout Sessions from `POST /checkout` |
| `STRIPE_WEBHOOK_SECRET` | Verify `POST /webhooks/stripe` |
| `STRIPE_WEBHOOK_PATH` | Optional mount path (default `/webhooks/stripe`) |
| `STRIPE_MOCK=1` | Mock checkout URL when no secret key (prototype only) |

## Admin auth

| Variable | Purpose |
|----------|---------|
| `ADMIN_TOKENS` | Comma-separated bearer tokens (rotation); **ops** unless also listed under support/finance |
| `ADMIN_TOKEN` | Legacy single token (prefer `ADMIN_TOKENS`) |
| `ADMIN_SUPPORT_TOKENS` | **support** role (read admin + cancel pending) |
| `ADMIN_FINANCE_TOKENS` | **finance** role (read-only admin) |

Secret handling: Stripe keys, webhook secrets, and bearer/HMAC material are redacted in structured logs (`pkg/redact`) and in audit / payment storage. Rotation: [rotate-admin-tokens](runbooks/rotate-admin-tokens.md), [rotate-core-api-hmac-keys](runbooks/rotate-core-api-hmac-keys.md).

## Gateway → API HMAC

| Variable | Purpose |
|----------|---------|
| `CORE_API_HMAC_SECRET` | Legacy v1 shared secret |
| `CORE_API_HMAC_KEY_ID` / `CORE_API_HMAC_KEYS` | v2 signing (`kid:secret` pairs; preferred) |

If **no** HMAC secret/keys are configured, signed routes accept unsigned requests (local prototyping only). Production must set keys — see [deploy](ops/deploy.md).

## SSH gateway

| Variable | Purpose |
|----------|---------|
| `SSH_ADDR` | Listen address (default `0.0.0.0:2222`) |
| `SSH_HOST_KEY_PATH` | Host key path (default `/data/ssh_host_ed25519_key`) |
| `SSH_REQUIRE_AUTHORIZED_KEYS` | `1` in production |
| `SSH_AUTHORIZED_KEYS_PATH` | Default `/data/authorized_keys` |
| `SSH_DEV_FINGERPRINT` | Dev-only fallback when no pubkey offered (**forbidden** with require-authorized-keys) |
| `SSH_DISABLE_PASSWORD_AUTH` | Default `1` in Compose |
| `SSH_DISABLE_PORT_FORWARDING` | Default `1` |
| `SIP_HAPPENS_SSH_PORT` | Host port published to container `2222` (dev/prod overlays) |

## Compliance URLs

| Variable | Default |
|----------|---------|
| `PRIVACY_POLICY_URL` | `https://sip-happens.local/privacy` |
| `CONSUMER_RIGHTS_URL` | `https://www.sic.gov.co/proteccion-del-consumidor` |
| `DATA_PROCESSING_POLICY_URL` | `https://sip-happens.local/legal/data-processing` — copy in [legal/data-processing.md](legal/data-processing.md) |
| `SUPPORT_CONTACT_EMAIL` | `soporte@sip-happens.local` (email footer unsubscribe) |

## Messaging / cache

| Variable | Purpose |
|----------|---------|
| `OUTBOX_PUBLISHER` | `nats` or `log` |
| `NAMESPACE` | JetStream subject namespace (`dev` / `staging` / `prod`) |
| `NATS_URL`, `NATS_STREAM`, `NATS_SUBJECT_PREFIX`, … | NATS / outbox |
| `REDIS_URL` | Optional; Compose sets `redis://redis:6379/0` |
| `REDIS_REQUIRED` | `1` → fail `/readyz` if Redis unavailable |
| `FEATURE_FLAGS` | See [Feature flags](#feature-flags) |

Outbox knobs: `OUTBOX_MAX_ATTEMPTS`, `OUTBOX_BASE_BACKOFF_SECONDS`, `OUTBOX_LOCKED_BY`, `OUTBOX_REPLAY_LIMIT`.

JetStream subjects (ADR 0016) — set `NAMESPACE` on `outbox-dispatch` and `nats-consume`:

| Kind | Pattern |
|------|---------|
| Events | `sip_happens.{NAMESPACE}.events.{event_type}` |
| Consumer filter | `sip_happens.{NAMESPACE}.events.>` |
| DLQ | `sip_happens.{NAMESPACE}.dlq` |

## Email

| Variable | Purpose |
|----------|---------|
| `EMAIL_PROVIDER` | `log` (default) or `smtp` |
| `EMAIL_FROM` | Default `noreply@sip-happens.local` |
| `SMTP_HOST`, `SMTP_PORT` (587), `SMTP_USER`, `SMTP_PASSWORD` | Required when `smtp` |

Transactional Spanish templates fire only when the matching feature flag is set and the customer has an email: payment link, receipt (`nats-consume`), shipping updates, newsletter confirm. Shared compliance footer: `_footer.{html,txt}`.

## Feature flags

Set on **core-api** and **nats-consume** (email receipt) via `FEATURE_FLAGS` (comma-separated, case-insensitive). Canonical list: `pkg/featureflags/known.go`.

| Flag | Service | Effect |
|------|---------|--------|
| `catalog_cache` | core-api | In-memory SWR cache for catalog GET (per replica) |
| `catalog_cache_redis` | core-api | Shared catalog cache in Redis; needs `REDIS_URL` |
| `redis_rate_limit` | core-api | Shared checkout/admin rate limits; needs `REDIS_URL`. See [rate-limit-scaling](ops/rate-limit-scaling.md) |
| `email_payment_link` | core-api | Email Stripe URL after `POST /checkout` |
| `email_receipt` | nats-consume | Receipt on `order.paid`; needs `OUTBOX_PUBLISHER=nats` |
| `email_shipping_update` | core-api | Email on shipment transitions |
| `email_subscribe_confirm` | core-api | Newsletter double-opt-in mail |
| `dian_invoice` | nats-consume | DIAN invoice row on `order.paid` (stub provider in dev) |

Example (typical Compose dev):

```bash
FEATURE_FLAGS=catalog_cache_redis,redis_rate_limit
```

## Object storage (MinIO)

- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET` (default `product-images`)
- `MINIO_PUBLIC_URL` — public base stored in `products.image_url`
- `MINIO_USE_SSL` — `1` when endpoint uses TLS
- Upload: `POST /admin/products/{id}/image` (`multipart/form-data`, field `image`, ops token)

## TLS (Caddy)

- `https://localhost:8443` → `core-api:8080` (`infra/caddy/Caddyfile`)
- Use for Stripe webhook forwarding and admin API over HTTPS in local dev (`curl -k`)

## Database

Compose sets `POSTGRES_DSN` for `core-api` / jobs to reach the `postgres` service.
