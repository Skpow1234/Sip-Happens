#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="infra/compose.yaml"

echo "Stopping containers..."
docker compose -f "$COMPOSE_FILE" down

