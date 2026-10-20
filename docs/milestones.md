# Milestones

GitHub **project milestones** on [Skpow1234/Sip-Happens](https://github.com/Skpow1234/Sip-Happens) are currently **empty**. This page tracks product status from the codebase and inline `roadmap-*` references (often paired with a private `docs/DESIGN.md`).

## Shipped (MVP shop)

| Area | Status |
|------|--------|
| SSH TUI (Bubble Tea) — age gate, catalog, cart, checkout QR/URL | Done |
| `core-api` catalog / cart / orders / addresses / subscriptions | Done |
| Stripe Checkout + webhooks + reconcile job + mock mode | Done |
| Inventory reservation + order reaper | Done |
| Outbox → NATS → fulfillment handoff | Done |
| Admin RBAC (ops / support / finance) — *roadmap-040* | Done |
| HMAC v2 gateway auth, PATs mint/revoke API, OAuth client + code grant | Done (PAT **Bearer use** incomplete — below) |
| Newsletter double-opt-in (Ley 1581) | Done |
| Shipping estimate + weight-tier table — *roadmap-140* | Done |
| Shipping-update email templates — *roadmap-138* | Done |
| Compliance email footer — *roadmap-139* | Done |
| DB metrics for expired pending orders — *roadmap-143* | Done |
| Compose dev / staging / prod overlays, CI, govulncheck | Done |
| Observability (`/metrics`, request IDs, healthchecks) | Done |
| Tracked migrations + demo SSH assets + OpenAPI (M1) | Done |

## Next (recommended order)

### M1 — Clone & CI completeness — **done**

- Tracked `infra/migrations/001`–`032` (linearity enforced in CI).
- Tracked demo SSH pubkeys / host key / `ssh_config` / `known_hosts` (`infra/demo/`; client private key still gitignored).
- Tracked `docs/openapi.yaml` aligned with `ImplementedRoutes`.

### M2 — Bearer PAT auth

- Wire middleware using `tokens.GetByPrefix` + hash verify; enforce scopes on cart/orders/profile.
- Unblocks documented “scripts / non-SSH API” and OAuth access-token use.
- Contract tests for Bearer vs HMAC paths.

### M3 — Production SSH shop live

- Follow [ops/deploy.md](ops/deploy.md): VM + Compose prod, published SSH port, `authorized_keys`, Caddy HTTPS, real Stripe.
- Pilot checklist referenced as *roadmap-150* (`scripts/stripe-webhook-smoke.sh`).

### M4 — Hardening

- Fail closed if HMAC keys missing when `NAMESPACE=prod`.
- Inventory / fulfillment package tests; coverage gate on money paths.
- Replace DIAN invoicing stub (`dian_invoice` / `INVOICE_PROVIDER`) for Colombia e-invoicing.

### M5 — Product depth

- More cities / shipping rules beyond Bogotá · Medellín · Cali.
- Draft order state (*ADR 0021*) if quote flows are needed.
- Split large `http` + TUI `model.go` packages for maintainability.

## Roadmap ID index (from code)

| ID | Topic | Location (examples) |
|----|--------|---------------------|
| 040 | Admin RBAC tiers | `pkg/config`, `admin_role.go` |
| 138 | Shipping status emails | `email/templates.go` |
| 139 | Compliance footer | `email/compliance.go` |
| 140 | Weight-tier shipping | `customers_cart_handlers.go`, `pkg/shipping` |
| 143 | Expired pending-order metrics | `observability/db_collector.go` |
| 150 | Pilot / post-deploy Stripe smoke | `scripts/stripe-webhook-smoke.sh` |

## Definition of “SSH available”

A deploy counts as **SSH-available** when:

1. `ssh-gateway` listens on a **host-reachable** TCP port.
2. `SSH_REQUIRE_AUTHORIZED_KEYS=1` and customer pubkeys are loaded.
3. HMAC secrets are set between gateway and API.
4. Age gate → catalog → cart → Stripe URL works end-to-end with `STRIPE_MOCK=0` (or mock only for staging).
