# Local development

Docker-only workflow: the host is assumed to have **only Docker** installed.

Base images are pinned by digest; CVE notes: [docker-image-security](ops/docker-image-security.md).

## Optional: local Go / IDE

`go.mod` requires **Go 1.25+** (`toolchain go1.25.10`). If the shell or editor still uses Go 1.24.x you will see `packages.Load` / `GOTOOLCHAIN=local` errors.

- **Cursor / VS Code:** set `GOTOOLCHAIN=auto` for gopls:

```json
{
  "go.toolsEnvVars": { "GOTOOLCHAIN": "auto" },
  "gopls": { "build.env": { "GOTOOLCHAIN": "auto" } }
}
```

- **Terminal:** `export GOTOOLCHAIN=auto` before `go test` / `go build`, or install Go 1.25+ system-wide.

Docker builds already set `GOTOOLCHAIN=auto`.

## One-command bring-up

```bash
bash scripts/dev-up.sh
```

Uses `infra/compose.yaml` + `infra/compose.dev.yaml` (publishes Postgres, Redis, NATS, MinIO, API, and SSH ports). Staging: [compose-environments](ops/compose-environments.md) and `./scripts/stack-up.sh staging`.

## Manual steps (same stack)

```bash
docker compose -f infra/compose.yaml -f infra/compose.dev.yaml build
docker compose -f infra/compose.yaml up -d postgres
docker compose -f infra/compose.yaml run --rm core-migrate up
docker compose -f infra/compose.yaml up -d core-api
```

### Stripe webhooks (signed, local)

`STRIPE_MOCK=1` (Compose default when no secret key) skips real Checkout. For end-to-end `POST /webhooks/stripe`, see [stripe-webhook-local](runbooks/stripe-webhook-local.md).

```bash
docker compose -f infra/compose.yaml run --rm stripe-cli \
  listen --forward-to http://core-api:8080/webhooks/stripe
```

Set `STRIPE_WEBHOOK_SECRET` from the CLI output, then recreate `core-api`.

### Background jobs

| Job | Command |
|-----|---------|
| Stripe reconcile (once) | `docker compose -f infra/compose.yaml run --rm -e JOB_RUN_ONCE=true core-jobs stripe-reconcile` |
| Stripe reconcile (loop) | `docker compose -f infra/compose.yaml up -d stripe-reconcile` |
| Order reaper | `docker compose -f infra/compose.yaml up -d order-reaper` |
| Outbox dispatch | `docker compose -f infra/compose.yaml -f infra/compose.dev.yaml up -d outbox-dispatch` |
| Inventory reconcile | `docker compose -f infra/compose.yaml run --rm core-jobs inventory-reconcile` |
| Fulfillment runner | `docker compose -f infra/compose.yaml up -d fulfillment-runner` |
| Outbox DLQ replay | `docker compose -f infra/compose.yaml run --rm core-jobs outbox-replay` |

`OUTBOX_PUBLISHER=nats` (Compose default) or `log` for structured log-only publish. NATS consumers: [nats-consumer-playbook](ops/nats-consumer-playbook.md).

Inventory reconcile knobs: `JOB_INTERVAL_SECONDS`, `JOB_BATCH_LIMIT`, `JOB_RUN_ONCE`, `INV_RECONCILE_FIX`, `INV_EXPIRE_GRACE_SECONDS`.

### Smoke

```bash
curl -fsS http://localhost:8080/healthz
# {"ok":true}
```

## SSH TUI (demo)

`dev-up` starts `ssh-gateway` and seeds a demo customer linked to the dev SSH key. **Public-key only** — no password.

```bash
ssh -F infra/demo/ssh_config sip-happens
```

If `infra/demo/sip-happens-demo` is missing (private key is gitignored):

```bash
ssh-keygen -t ed25519 -f infra/demo/sip-happens-demo -N "" -C sip-happens-demo@local
```

Matching public key: `infra/demo/authorized_keys`. Fixed dev host key avoids `REMOTE HOST IDENTIFICATION HAS CHANGED` on recreate. See `infra/demo/README.md` when that tree is present.

Dev HMAC is pre-wired: `CORE_API_HMAC_KEY_ID=dev`, `CORE_API_HMAC_KEYS=dev:dev-hmac-secret`.

TUI details: [tui.md](tui.md).

## Stop / clean

```bash
bash scripts/dev-down.sh    # keep volumes
bash scripts/dev-reset.sh   # wipe volumes / local DB data
```
