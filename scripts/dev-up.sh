#!/usr/bin/env bash
# shellcheck disable=SC2086
# Word-splitting on $COMPOSE_FILES is intentional: it expands to
# `-f infra/compose.yaml -f infra/compose.dev.yaml` and must remain
# unquoted so docker compose receives two separate -f flags.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILES="-f infra/compose.yaml -f infra/compose.dev.yaml"

# Only pull images from a registry. Do not use "compose pull" without a service list:
# jobs like core-migrate use image: infra-core-api (local build tag), which is not on Docker Hub.
# Set DEV_UP_SKIP_PULL=1 for faster iteration when offline or images are already current.
INFRA_IMAGES=(postgres nats redis minio caddy)
if [[ "${DEV_UP_SKIP_PULL:-}" != "1" ]]; then
  echo "Pulling latest infra images (${INFRA_IMAGES[*]})..."
  docker compose $COMPOSE_FILES pull "${INFRA_IMAGES[@]}"
fi

echo "Building application images (core-api, ssh-gateway)..."
docker compose $COMPOSE_FILES build --pull

echo "Starting infra (postgres, nats, redis, minio)..."
docker compose $COMPOSE_FILES up -d --pull always postgres nats redis minio
echo "Initializing MinIO bucket..."
docker compose $COMPOSE_FILES run --rm minio-init

echo "Running migrations..."
docker compose $COMPOSE_FILES run --rm core-migrate up

echo "Starting services..."
docker compose $COMPOSE_FILES up -d --force-recreate \
  core-api ssh-gateway order-reaper stripe-reconcile outbox-dispatch nats-consume fulfillment-runner caddy

echo "Seeding demo data (safe to re-run)..."
docker compose $COMPOSE_FILES run --rm core-api seed demo

echo "Done."
echo "HTTP:  http://localhost:8080/healthz"
echo "HTTPS: https://localhost:8443/healthz  (Caddy TLS; curl -k)"
echo "MinIO: http://localhost:9000  console http://localhost:9001"
echo "SSH:   ssh -F infra/demo/ssh_config sip-happens"
echo "       (demo keypair tracked under infra/demo/ — local-only)"
echo ""
echo "Stripe webhooks (optional E2E): dev-up does not start stripe-cli (needs STRIPE_API_KEY)."
echo "  Runbook: docs/runbooks/stripe-webhook-local.md"
echo "  Forward: docker compose $COMPOSE_FILES run --rm stripe-cli \\"
echo "    listen --forward-to http://core-api:8080/webhooks/stripe"
echo "  Copy the webhook signing secret into STRIPE_WEBHOOK_SECRET, then recreate core-api."

