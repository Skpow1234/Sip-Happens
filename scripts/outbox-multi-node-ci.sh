#!/usr/bin/env bash
# CI smoke: two outbox-dispatch workers claim the same pending batch without duplicates.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -z "${POSTGRES_DSN:-}" ]]; then
  echo "POSTGRES_DSN is required" >&2
  exit 1
fi

COUNT="${OUTBOX_SMOKE_COUNT:-50}"
RUN_ID="${OUTBOX_SMOKE_RUN_ID:-ci-$(date +%s)}"
export OUTBOX_PUBLISHER=log
export JOB_RUN_ONCE=true
export JOB_BATCH_LIMIT="${JOB_BATCH_LIMIT:-40}"
export EMAIL_PROVIDER=log

echo "outbox multi-node smoke: run_id=$RUN_ID count=$COUNT"

go run ./services/core-api/cmd/outbox-multi-node-smoke seed --count="$COUNT" --run-id="$RUN_ID"

dispatch_round() {
  OUTBOX_LOCKED_BY=worker-a go run ./services/core-api/cmd/jobs outbox-dispatch &
  pid_a=$!
  OUTBOX_LOCKED_BY=worker-b go run ./services/core-api/cmd/jobs outbox-dispatch &
  pid_b=$!
  wait "$pid_a" "$pid_b"
}

for _ in $(seq 1 8); do
  pending="$(go run ./services/core-api/cmd/outbox-multi-node-smoke pending --run-id="$RUN_ID" --count=0)"
  if [[ "$pending" == "0" ]]; then
    break
  fi
  dispatch_round
done

go run ./services/core-api/cmd/outbox-multi-node-smoke verify --count="$COUNT" --run-id="$RUN_ID"
echo "outbox multi-node smoke: ok"
