# Compose environments

| Overlay | Purpose |
|---------|---------|
| `infra/compose.yaml` | Base stack (internal network; no host DB ports) |
| `infra/compose.dev.yaml` | Publish Postgres/Redis/NATS/MinIO/API/SSH for local work |
| `infra/compose.staging.yaml` | `NAMESPACE=staging` on API / outbox / consumers |
| `infra/compose.prod.yaml` | Secrets via env file, restart policy, no published API port, SSH published, authorized_keys required |

## Dev

```bash
bash scripts/dev-up.sh
# or:
docker compose -f infra/compose.yaml -f infra/compose.dev.yaml up -d
```

## Staging

```bash
COMPOSE_PROJECT_NAME=sip-happens-staging \
  docker compose -f infra/compose.yaml -f infra/compose.staging.yaml --env-file .env.staging up -d
```

To publish host ports beside a running dev stack, also add `-f infra/compose.dev.yaml` and set `SIP_HAPPENS_HTTP_PORT` / `SIP_HAPPENS_SSH_PORT` in `.env.staging`. Helper: `./scripts/stack-up.sh staging`.

## Production

```bash
cp infra/env.prod.example .env.prod
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod up -d
```

See [deploy.md](deploy.md).
