# Incident: Stripe webhook outage

## Symptoms

Orders stuck in `PENDING_PAYMENT` despite successful Checkout; webhook delivery failures in Stripe Dashboard; `/webhooks/stripe` 5xx or signature errors.

## Mitigations

1. Confirm `STRIPE_WEBHOOK_SECRET` matches the endpoint in Stripe; path matches `STRIPE_WEBHOOK_PATH`.
2. Check Caddy / TLS path to `core-api`; scrape API logs for signature failures.
3. Run reconcile to catch up:

```bash
docker compose -f infra/compose.yaml run --rm -e JOB_RUN_ONCE=true core-jobs stripe-reconcile
# or admin: POST /admin/orders/{id}/reconcile-stripe with Idempotency-Key
```

4. Replay Stripe DLQ if events landed there: `GET/POST /admin/stripe/dead-letters…` (ops).
5. Keep `stripe-reconcile` worker running so a short outage self-heals.
