# Runbook: rotate admin tokens

1. Generate a new high-entropy token; add it to `ADMIN_TOKENS` (comma-separated) **alongside** the old one.
2. Redeploy / recreate `core-api` so both tokens work.
3. Update ops clients (console, scripts, CI secrets) to the new token.
4. Remove the old token from `ADMIN_TOKENS` (and from `ADMIN_SUPPORT_TOKENS` / `ADMIN_FINANCE_TOKENS` if listed).
5. Redeploy again; confirm old token returns 401.
6. Prefer role splits: support/finance tokens instead of sharing full ops tokens.
