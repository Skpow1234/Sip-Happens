# Runbook: rotate SSH host key

1. Schedule a maintenance window — clients will see host key changed.
2. Generate a new ed25519 host key; place at `SSH_HOST_KEY_PATH` (volume `sshkeys` or bind mount).
3. Recreate `ssh-gateway`.
4. Publish the new fingerprint out-of-band; ask customers to update `known_hosts` (or use a documented `ssh-keygen -R` + re-accept flow).
5. Confirm `ssh -p …` connects without MITM warnings for operators who installed the new key.
