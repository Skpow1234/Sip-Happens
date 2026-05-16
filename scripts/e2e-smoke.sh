#!/usr/bin/env bash
# Minimal HTTP smoke against a running core-api (health, catalog, signed routes, checkout).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export BASE_URL="${BASE_URL:-http://localhost:8080}"
export SSH_FINGERPRINT="${SSH_FINGERPRINT:-demo:ssh-fingerprint}"
export CORE_API_HMAC_KEY_ID="${CORE_API_HMAC_KEY_ID:-dev}"
export CORE_API_HMAC_KEYS="${CORE_API_HMAC_KEYS:-dev:dev-hmac-secret}"
cd "$ROOT"
exec go run ./services/core-api/cmd/e2e-smoke
