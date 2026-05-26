#!/usr/bin/env bash
# stripe-webhook-smoke.sh
#
# Sends a synthetic, properly-signed Stripe webhook to the running core-api
# so an operator can verify the prod webhook path end-to-end without paying
# real money through Stripe Checkout. Designed for the prod COP sign-off
# (roadmap-150 pilot checklist) and the post-deploy smoke gate.
#
# What it does:
#   1. Generates a Stripe-compatible JSON event payload for
#      `checkout.session.completed` referencing the order id and PI id you
#      pass in.
#   2. Computes the `Stripe-Signature: t=<ts>,v1=<HMAC-SHA256(secret, ts.payload)>`
#      header that webhook.go's StripeGoVerifier accepts.
#   3. POSTs to ${BASE_URL}${STRIPE_WEBHOOK_PATH} (default http://localhost:8080/webhooks/stripe).
#   4. Asserts a 2xx response and exits non-zero on anything else.
#
# Required env:
#   STRIPE_WEBHOOK_SECRET  — same value the running core-api has loaded.
#   SMOKE_ORDER_ID         — an order id that exists in the local DB in
#                            PENDING_PAYMENT (run e2e-smoke first to create one).
#
# Optional env:
#   BASE_URL              default http://localhost:8080
#   STRIPE_WEBHOOK_PATH   default /webhooks/stripe
#   SMOKE_PI_ID           default pi_smoke_<timestamp>
#   SMOKE_SESSION_ID      default cs_smoke_<timestamp>
#   SMOKE_AMOUNT_MINOR    default 100000  (1000 COP)
#   SMOKE_CURRENCY        default cop
#
# Exit codes:
#   0  webhook delivered, core-api ack'd 2xx
#   1  configuration error (missing env / openssl)
#   2  HTTP non-2xx (core-api rejected — check logs)

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
STRIPE_WEBHOOK_PATH="${STRIPE_WEBHOOK_PATH:-/webhooks/stripe}"
SMOKE_PI_ID="${SMOKE_PI_ID:-pi_smoke_$(date +%s)}"
SMOKE_SESSION_ID="${SMOKE_SESSION_ID:-cs_smoke_$(date +%s)}"
SMOKE_AMOUNT_MINOR="${SMOKE_AMOUNT_MINOR:-100000}"
SMOKE_CURRENCY="${SMOKE_CURRENCY:-cop}"

if [[ -z "${STRIPE_WEBHOOK_SECRET:-}" ]]; then
  echo "ERROR: STRIPE_WEBHOOK_SECRET is required (same value core-api has loaded)." >&2
  exit 1
fi
if [[ -z "${SMOKE_ORDER_ID:-}" ]]; then
  echo "ERROR: SMOKE_ORDER_ID is required (id of an existing PENDING_PAYMENT order)." >&2
  echo "       Hint: run scripts/e2e-smoke.sh first; it creates one and prints its id." >&2
  exit 1
fi
if ! command -v openssl >/dev/null 2>&1; then
  echo "ERROR: openssl not on PATH; this script uses 'openssl dgst -sha256 -hmac'." >&2
  exit 1
fi

TS="$(date +%s)"

# Stripe-compatible event payload. Fields the webhook actually reads:
#   data.object.id, data.object.payment_intent, data.object.metadata.order_id.
# Anything else is ignored.
PAYLOAD=$(cat <<EOF
{
  "id": "evt_smoke_${TS}",
  "object": "event",
  "type": "checkout.session.completed",
  "created": ${TS},
  "data": {
    "object": {
      "id": "${SMOKE_SESSION_ID}",
      "object": "checkout.session",
      "payment_intent": "${SMOKE_PI_ID}",
      "amount_total": ${SMOKE_AMOUNT_MINOR},
      "currency": "${SMOKE_CURRENCY}",
      "metadata": {
        "order_id": "${SMOKE_ORDER_ID}"
      },
      "payment_status": "paid",
      "status": "complete"
    }
  }
}
EOF
)

SIGNED_PAYLOAD="${TS}.${PAYLOAD}"
SIG=$(printf '%s' "${SIGNED_PAYLOAD}" \
  | openssl dgst -sha256 -hmac "${STRIPE_WEBHOOK_SECRET}" -hex \
  | sed 's/^.* //')

STRIPE_SIG="t=${TS},v1=${SIG}"

URL="${BASE_URL}${STRIPE_WEBHOOK_PATH}"
echo "POST ${URL}"
echo "  order_id=${SMOKE_ORDER_ID}"
echo "  session=${SMOKE_SESSION_ID}"
echo "  pi=${SMOKE_PI_ID}"
echo "  amount=${SMOKE_AMOUNT_MINOR} ${SMOKE_CURRENCY}"

HTTP_CODE=$(curl -sS -o /tmp/stripe-webhook-smoke.out -w '%{http_code}' \
  -X POST "${URL}" \
  -H 'Content-Type: application/json' \
  -H "Stripe-Signature: ${STRIPE_SIG}" \
  --data-binary "${PAYLOAD}")

echo "← HTTP ${HTTP_CODE}"
cat /tmp/stripe-webhook-smoke.out
echo

case "${HTTP_CODE}" in
  2*)
    echo "stripe-webhook-smoke: ok"
    exit 0
    ;;
  *)
    echo "stripe-webhook-smoke: FAILED (HTTP ${HTTP_CODE}) — check core-api logs and stripe_dead_letters table." >&2
    exit 2
    ;;
esac
