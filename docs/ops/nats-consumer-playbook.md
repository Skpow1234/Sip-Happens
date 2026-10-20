# NATS consumer playbook

## Layout (ADR 0016)

Set the same `NAMESPACE` on `outbox-dispatch` and `nats-consume`.

| Kind | Subject |
|------|---------|
| Events | `sip_happens.{NAMESPACE}.events.{event_type}` |
| Filter | `sip_happens.{NAMESPACE}.events.>` |
| DLQ | `sip_happens.{NAMESPACE}.dlq` |

## Dev

`bash scripts/dev-up.sh` starts `outbox-dispatch`, `nats-consume` (`orders.order.paid` → shipment), and `fulfillment-runner`.

Log-only outbox (no JetStream publish):

```bash
OUTBOX_PUBLISHER=log docker compose -f infra/compose.yaml -f infra/compose.dev.yaml up -d outbox-dispatch
```

## Ops tips

- Backlog: scrape `/metrics` outbox gauges; check `outbox_events` / `outbox_dead_letters` in Postgres.
- Replay DLQ: `docker compose … run --rm core-jobs outbox-replay` (`OUTBOX_REPLAY_LIMIT`).
- Multi-node claim smoke: `bash scripts/outbox-multi-node-ci.sh`.
- If consumers are idle: confirm `OUTBOX_PUBLISHER=nats`, `NATS_URL`, matching `NAMESPACE`, and that JetStream stream `NATS_STREAM` exists.
