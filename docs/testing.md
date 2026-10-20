# Testing

## Unit

No external dependencies:

```bash
go test ./...
```

DB-backed tests skip when `POSTGRES_DSN` is unset.

## Integration (Postgres)

```bash
POSTGRES_DSN="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" go test ./...
```

CI runs race-enabled `./services/core-api/...` against a Postgres service.

## E2E smoke

Needs migrations + demo seed:

```bash
go run ./services/core-api/cmd/api seed demo
# start core-api (Compose or HTTP_ADDR=:8080 go run ./services/core-api/cmd/api)
bash scripts/e2e-smoke.sh
```

CI: `go run ./services/core-api/cmd/e2e-smoke` (gated on `infra/migrations/*.sql` existing).

## Load / concurrency

```bash
go test ./services/core-api/internal/orders/... -run TestConcurrentCreatePendingOrder -count=1
go test ./services/core-api/internal/orders/... -bench=BenchmarkCreatePendingOrder_Concurrent -benchtime=2s -run=^$
```

## Outbox multi-node

```bash
bash scripts/outbox-multi-node-ci.sh
```

Two `outbox-dispatch` workers against a shared DB.
