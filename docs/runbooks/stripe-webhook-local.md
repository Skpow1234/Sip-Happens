# Runbook: Stripe webhooks locally

1. `bash scripts/dev-up.sh` (or equivalent Compose up).
2. Set `STRIPE_API_KEY` for the CLI container and run:

```bash
docker compose -f infra/compose.yaml run --rm stripe-cli \
  listen --forward-to http://core-api:8080/webhooks/stripe
```

3. Copy the signing secret (`whsec_…`) into `STRIPE_WEBHOOK_SECRET` on `core-api`.
4. Recreate the API: `docker compose -f infra/compose.yaml up -d --force-recreate core-api`.
5. Complete a Checkout (disable `STRIPE_MOCK` / set real `STRIPE_SECRET_KEY` as needed) and confirm `POST /webhooks/stripe` marks the order paid.

Optional smoke: `bash scripts/stripe-webhook-smoke.sh` (pilot / post-deploy gate).
