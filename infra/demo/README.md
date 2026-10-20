# Demo SSH assets (local only)

Used by `compose.dev.yaml` and `bash scripts/dev-up.sh`.

| File | Purpose |
|------|---------|
| `sip-happens-demo` (+ `.pub`) | Customer keypair (empty passphrase, **local-only**) |
| `ssh_host_ed25519_key` (+ `.pub`) | Fixed host key so recreates do not break `known_hosts` |
| `authorized_keys` | Mounted into `ssh-gateway` (contains the demo pubkey) |
| `ssh_config` | Convenience client config |
| `known_hosts` | Pins the demo host key for localhost:2222 |

Fingerprint baked into Compose:  
`SSH_DEV_FINGERPRINT=SHA256:kqjoLaSTFR7d2Vg62ebJEeM6hwJCbJcLzqvGnBxFQp8`

If you **regenerate** the customer keypair, update `.pub`, `authorized_keys`, and that env default together.

## Connect

From the repo root:

```bash
ssh -F infra/demo/ssh_config sip-happens
```
