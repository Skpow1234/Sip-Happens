# Testing

## CI map (GitHub Actions)

Workflow: [`.github/workflows/ci.yml`](../.github/workflows/ci.yml). Require the single
check named **CI** in branch protection (it aggregates the jobs below).

| Job | What it gates |
|-----|----------------|
| `lint` | gofmt, vet, staticcheck, `go mod tidy`, migration linearity, OpenAPI YAML parse, shellcheck |
| `unit` | race tests for `pkg/…` + `ssh-gateway` (no Postgres) |
| `security` | `govulncheck` on shipped entrypoints (blocking); full-tree advisory |
| `build` | `CGO_ENABLED=0` compile of api / jobs / migrate / ssh-gateway |
| `integration` | race `core-api` tests + migrate up + outbox multi-node + HTTP e2e smoke |
| `docker` | Buildx images with GHA layer cache + Trivy CRITICAL/HIGH + compose config |
| `CI` | Passes only when every job above is `success` |

Release images (tag `v*` or manual): [`.github/workflows/release.yml`](../.github/workflows/release.yml) → GHCR.

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

CI starts core-api, waits for `/healthz` (fails if the process dies or the wait times out), then runs `go run ./services/core-api/cmd/e2e-smoke`.

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
