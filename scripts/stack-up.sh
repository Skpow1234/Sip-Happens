#!/usr/bin/env bash
set -euo pipefail

STACK="${1:-dev}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

case "$STACK" in
  dev)
    export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-sip-happens-dev}"
    exec "$ROOT/scripts/dev-up.sh"
    ;;
  staging)
    export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-sip-happens-staging}"
    COMPOSE_FILES="-f infra/compose.yaml -f infra/compose.staging.yaml"
    ENV_FILE="${SIP_HAPPENS_ENV_FILE:-.env.staging}"
    if [[ -f "$ENV_FILE" ]]; then
      COMPOSE_ENV=(--env-file "$ENV_FILE")
    else
      echo "hint: copy infra/env.staging.example to .env.staging"
      COMPOSE_ENV=()
    fi
    ;;
  *)
    echo "usage: $0 {dev|staging}" >&2
    exit 1
    ;;
esac

echo "Building ($STACK, project=$COMPOSE_PROJECT_NAME)..."
docker compose "${COMPOSE_ENV[@]}" $COMPOSE_FILES build

echo "Starting infra..."
docker compose "${COMPOSE_ENV[@]}" $COMPOSE_FILES up -d postgres nats redis minio
docker compose "${COMPOSE_ENV[@]}" $COMPOSE_FILES run --rm minio-init

echo "Migrating..."
docker compose "${COMPOSE_ENV[@]}" $COMPOSE_FILES run --rm core-migrate up

echo "Starting services..."
docker compose "${COMPOSE_ENV[@]}" $COMPOSE_FILES up -d core-api ssh-gateway order-reaper stripe-reconcile outbox-dispatch nats-consume fulfillment-runner caddy

echo "Seeding demo..."
docker compose "${COMPOSE_ENV[@]}" $COMPOSE_FILES run --rm core-api seed demo

echo "Done ($STACK). See docs/ops/compose-environments.md"
