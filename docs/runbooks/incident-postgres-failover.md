# Incident: Postgres unavailable / failover

## Symptoms

`/readyz` 503; API errors `misconfigured` / DB dial errors; workers crash-looping on `POSTGRES_DSN`.

## Actions

1. Confirm Postgres health (`pg_isready`, Compose healthcheck). Restore from backup / promote replica per your hoster if using managed Postgres.
2. Update `POSTGRES_DSN` on `core-api` and all `core-jobs` services; recreate them.
3. Do **not** expose Postgres on the public internet while recovering.
4. After DB is up: run migrate if schema drifted; check outbox backlog and pending payments (`stripe-reconcile`, `order-reaper`).
5. Verify `/readyz` and a signed smoke (`/view/init` via gateway or e2e-smoke against staging).
