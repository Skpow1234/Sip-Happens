# Rate-limit scaling

Feature flag `redis_rate_limit` moves checkout and admin token-bucket limits from in-process memory to Redis (`REDIS_URL`).

## When to enable

- More than one `core-api` replica, or
- You need shared limits across rolling deploys.

Without the flag, each replica has its own bucket (effective limit ≈ N × configured rate).

## Requirements

- `REDIS_URL` reachable from every `core-api` instance
- Prefer `REDIS_REQUIRED=1` in prod so `/readyz` fails if Redis is down while limits are required for abuse protection

## Ops

- Monitor Redis latency and saturation; rate-limit failures should surface as HTTP 429 with the standard error envelope.
- Rotate Redis independently of HMAC/admin secrets.
