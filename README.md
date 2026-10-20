# Sip-Happens

Terminal-first beer shop for Colombia.

Users connect via **SSH** to a TUI, browse a beer catalog, build a cart, and start checkout. **Payments happen off-terminal** via **Stripe-hosted checkout** (no card entry in SSH). The backend owns order state, inventory reservations, auditability, and compliance (age gate, INVIMA visibility, Ley 1581).

## What’s in this repo

| Piece | Role |
|-------|------|
| **`core-api`** | Go HTTP API + Postgres: catalog, cart, orders, subscriptions, admin, Stripe webhooks |
| **`ssh-gateway`** | Bubble Tea TUI over SSH (shop, account, cart, off-terminal payment) |
| **`infra/`** | Docker Compose (dev / staging / prod), Caddy, env examples |
| **`docs/`** | [Full documentation index](docs/README.md) |

Docker-only workflow: the host is assumed to have **only Docker** installed.

## Quick start (local)

```bash
bash scripts/dev-up.sh
ssh -F infra/demo/ssh_config sip-happens
curl -fsS http://localhost:8080/healthz
```

Demo SSH keypair is tracked under `infra/demo/` (local-only). Details: [Local development](docs/local-development.md) · [TUI shortcuts](docs/tui.md)

## Deploy so customers can SSH in

Production Compose publishes the shop SSH port and requires authorized keys:

```bash
cp infra/env.prod.example .env.prod   # fill secrets, SIP_HAPPENS_SSH_PORT, Stripe, HMAC…
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod up -d
```

Then: DNS → VM, firewall for SSH + HTTPS, load `authorized_keys`, point Stripe webhooks at Caddy.

**Guide:** [Deploy (SSH shop online)](docs/ops/deploy.md) · [SSH production](docs/ops/ssh-production.md)

## Docs map

| Topic | Link |
|-------|------|
| API endpoints | [docs/endpoints.md](docs/endpoints.md) |
| Environment & feature flags | [docs/environment.md](docs/environment.md) |
| Milestones / backlog | [docs/milestones.md](docs/milestones.md) (M1–M2 done; M3 next; 900+ backlog) |
| Testing & CI | [docs/testing.md](docs/testing.md) (require GitHub check **CI**) |
| Observability | [docs/observability.md](docs/observability.md) |
| Compose environments | [docs/ops/compose-environments.md](docs/ops/compose-environments.md) |
| Runbooks (incidents, rotations) | [docs/runbooks/](docs/runbooks/) |
| Legal (Ley 1581 draft) | [docs/legal/data-processing.md](docs/legal/data-processing.md) |

## Stop / clean

```bash
bash scripts/dev-down.sh    # keep volumes
bash scripts/dev-reset.sh   # wipe volumes
```

## Optional: local Go

`go.mod` needs **Go 1.26+** (`toolchain go1.26.10`). Use `GOTOOLCHAIN=auto` in the shell or editor if the host Go is older. Docker builds already set this.
