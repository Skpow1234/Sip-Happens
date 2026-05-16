#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="infra/compose.yaml"

echo "This will remove containers and volumes (DB data)."
echo "Press Ctrl+C to cancel, or wait 3 seconds..."
sleep 3

docker compose -f "$COMPOSE_FILE" down -v

echo "Reset complete."

