# Architecture Decision Records

Referenced from code and docs. Full ADR markdown may live in a private design pack (`docs/DESIGN.md` is gitignored) or be added here over time.

| ID | Title | Where it shows up |
|----|--------|-------------------|
| 0006 | Domain event envelope / types | `pkg/events` |
| 0016 | NATS JetStream subject layout | `pkg/events/nats_subjects.go`, outbox / consumers |
| 0020 | Terminal.shop API parity (Colombia + SSH + HMAC, no cards in SSH) | README, OpenAPI goals |
| 0021 | Draft order state (`OrderStatusDraft`) | `pkg/domain/order.go` |

When adding a new ADR, drop `NNNN-short-title.md` in this folder and link it from the table.
