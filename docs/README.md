# Documentation

Operator and developer docs for Sip-Happens. Start with the root [README](../README.md) for a one-screen overview, then jump here for detail.

| Doc | What it covers |
|-----|----------------|
| [Local development](local-development.md) | Docker-only bring-up, SSH TUI demo, Stripe webhooks, jobs |
| [Deploy (SSH shop online)](ops/deploy.md) | Production / staging Compose, DNS, firewall, go-live checklist |
| [SSH production](ops/ssh-production.md) | Host keys, `authorized_keys`, fingerprints |
| [API endpoints](endpoints.md) | HTTP surface (customer, admin, webhooks) |
| [Environment variables](environment.md) | Secrets, feature flags, MinIO, email |
| [TUI](tui.md) | Screens, shortcuts, UX notes |
| [Observability](observability.md) | Metrics, readiness, Grafana / alerts |
| [Testing](testing.md) | Unit, integration, e2e, concurrency |
| [Milestones](milestones.md) | M1–M5 + 900+ engineering backlog items |
| [Compose environments](ops/compose-environments.md) | Dev / staging / prod overlays |
| [Legal](legal/data-processing.md) | Ley 1581 draft + retention |
| [ADRs](adr/README.md) | Architecture decision index |
| [Runbooks](runbooks/) | Incidents, rotations, Stripe local |

OpenAPI contract: [`openapi.yaml`](openapi.yaml) — also served at `GET /openapi.yaml`.
