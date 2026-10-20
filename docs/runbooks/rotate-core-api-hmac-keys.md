# Runbook: rotate core-api HMAC keys

Preferred format: `CORE_API_HMAC_KEYS=kid1:secret1,kid2:secret2` with `CORE_API_HMAC_KEY_ID` selecting the signing kid on `ssh-gateway`.

1. Add a new `kid:secret` pair to `CORE_API_HMAC_KEYS` on **both** `core-api` and `ssh-gateway` (API must accept old + new during overlap).
2. Point `CORE_API_HMAC_KEY_ID` on the gateway to the new kid; recreate gateway.
3. Confirm TUI traffic succeeds (signed calls).
4. Drop the old kid from `CORE_API_HMAC_KEYS` on API + gateway; recreate.
5. Do not log secrets; `pkg/redact` should already scrub them.
