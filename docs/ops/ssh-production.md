# SSH production

Customers authenticate with **SSH public keys only**. Each key maps to a stable customer fingerprint (`SHA256:…`, OpenSSH-compatible — same as `ssh-keygen -lf`).

## Requirements

| Setting | Production value |
|---------|------------------|
| `SSH_REQUIRE_AUTHORIZED_KEYS` | `1` |
| `SSH_AUTHORIZED_KEYS_PATH` | `/data/authorized_keys` (default) |
| `SSH_DISABLE_PASSWORD_AUTH` | `1` |
| `SSH_DISABLE_PORT_FORWARDING` | `1` |
| `SSH_DEV_FINGERPRINT` | **unset** (config refuses it when require-authorized-keys is on) |
| Host key | Persistent volume `sshkeys` → `/data/ssh_host_ed25519_key` |

Local dev mounts a committed host key via `compose.dev.yaml`. Production uses the named volume so recreates keep the same host identity. Rotation: [rotate-ssh-host-key](../runbooks/rotate-ssh-host-key.md).

## Authorized keys

Template: [`infra/authorized_keys.example`](../../infra/authorized_keys.example).

```bash
ssh-keygen -t ed25519 -f sip-happens-customer -N "" -C customer@example.com
cat sip-happens-customer.pub >> authorized_keys
ssh-keygen -lf sip-happens-customer.pub   # → SHA256:… fingerprint used as customer id
```

### Inject into Compose volume

After the stack has created volume `sshkeys`:

```bash
# Example: copy into a running gateway (adjust container name from `docker compose ps`)
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod cp \
  ./authorized_keys ssh-gateway:/data/authorized_keys
docker compose -f infra/compose.yaml -f infra/compose.prod.yaml --env-file .env.prod restart ssh-gateway
```

Or bind-mount a host file in an override:

```yaml
services:
  ssh-gateway:
    volumes:
      - /etc/sip-happens/authorized_keys:/data/authorized_keys:ro
```

## Client connect

```bash
ssh -p 2222 -i ~/.ssh/sip-happens-customer shop.example.com
```

First connect: verify the host key fingerprint out-of-band (ops channel), then accept into `known_hosts`.

## Gateway → API

`ssh-gateway` calls `core-api` with HMAC v2 (`CORE_API_HMAC_KEY_ID` / `CORE_API_HMAC_KEYS`) and `X-SSH-Fingerprint`. Keep those secrets aligned with the API and rotate via [rotate-core-api-hmac-keys](../runbooks/rotate-core-api-hmac-keys.md).

## Publish the port

`infra/compose.prod.yaml` maps `${SIP_HAPPENS_SSH_PORT:-2222}:2222`. Without that publish (or an equivalent LB TCP forward), the TUI is unreachable from the internet even if the container is healthy. See [deploy.md](deploy.md).
