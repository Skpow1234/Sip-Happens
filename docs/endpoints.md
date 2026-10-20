# API endpoints

Full machine-readable contract: [`openapi.yaml`](openapi.yaml) (also `GET /openapi.yaml`; Redoc at `GET /docs` with admin Bearer token). Terminal.shop parity rationale: [ADR 0020](adr/README.md#0020-terminal-shop-api-parity).

Public ids in JSON use stable prefixes: `prd_…` (product), `addr_…` (address), `pm_…` (Stripe payment method), `sub_…` (subscription), `pat_…` (personal access token), `oc_…` / `oci_…` (OAuth client). Orders also expose a human-readable `display_ref` (`SH-…`) accepted on `/orders/{id}`.

## Error envelope

All API errors return JSON in a stable shape:

```json
{
  "error": {
    "code": "invalid_input",
    "message": "request invalid",
    "request_id": "..."
  }
}
```

## Public

- `GET /healthz`, `GET /readyz` (DB readiness; 503 until DB is reachable), `GET /metrics`
- `GET /catalog?currency=COP` — list active products with price and `available_qty` (`limit`, `offset`, `q`, `style`, `brewery`, `abv_min`, `abv_max`; `featured=true` for destacados)
- `GET /catalog/{id}?currency=COP` — product detail (`prd_…` public id, UUID, or SKU) with `available_qty`
- `GET /openapi.yaml` — OpenAPI 3 spec

## Signed — profile

HMAC v2 + `X-SSH-Fingerprint` (used by `ssh-gateway`).

- `GET /customers/me`, `PATCH /customers/me` — profile (snake_case JSON)
- `GET /profile`, `PUT /profile`, `PATCH /profile` — Terminal.shop aliases (`profile` wrapper)

## Signed — addresses

- `GET /addresses`, `POST /addresses`, `GET /addresses/{id}`, `DELETE /addresses/{id}` — saved delivery addresses (Bogotá, Medellín, Cali)

## Signed — payment methods

Stripe-hosted; **no card data ever crosses SSH**.

- `POST /payment-methods/collect` — start a hosted Stripe Setup/Checkout Session to save a card; returns `{ collect: { url, checkout_session_id } }`
- `GET /payment-methods` — list saved cards on the customer’s Stripe customer
- `GET /payment-methods/{id}` — get one (`pm_…`), 404 when not owned
- `DELETE /payment-methods/{id}` — detach saved card

## Signed — cart

- `GET /cart` — server-persistent cart with live prices, subtotal, and optional saved `address_id` / `payment_method_id`
- `PUT /cart` — bulk replace cart lines
- `DELETE /cart` — clear all lines, saved `address_id`, and `payment_method_id`
- `PUT /cart/items/{id}` — set quantity for one product (`qty: 0` removes); `id` accepts `prd_…`, UUID, or SKU
- `PUT /cart/address` — attach saved `address_id` (empty string clears)
- `PUT /cart/payment-method` — attach saved Stripe `payment_method_id` (empty string clears)
- `POST /cart/convert` — convert cart → `PENDING_PAYMENT` + Stripe Checkout URL (`Idempotency-Key` required); clears cart on success

## Signed — orders

- `GET /orders` — list caller’s orders (`limit`, default 20, max 100; `offset`, `has_more`); no delivery PII in list view
- `POST /orders` — direct checkout from `items` + delivery (no server cart) → Stripe Checkout URL (`Idempotency-Key` required); does **not** touch the server cart
- `GET /orders/{id}` — order detail (internal UUID or `display_ref`; scoped to caller’s fingerprint)
- `POST /orders/{ref}/cancel` — customer cancels their own `PENDING_PAYMENT` order (optional `reason` body, ≤120 chars); idempotent. Post-`PENDING_PAYMENT` orders return 409 — request an admin refund instead.

## Signed — subscriptions

Stripe Billing; Checkout in subscription mode.

- `GET /subscriptions` — list (`limit`, `offset`, `has_more`)
- `POST /subscriptions` — create `PENDING` + hosted subscription Checkout URL (`Idempotency-Key` required; body: `product_id`, `qty`, `interval` (`week`|`month`), optional `address_id`, `payment_method_id`, success/cancel URLs)
- `GET /subscriptions/{id}` — detail (`sub_…` or internal UUID)
- `PUT /subscriptions/{id}` — update `qty` and/or `address_id`
- `DELETE /subscriptions/{id}` — cancel (cancels Stripe subscription when linked); idempotent

## Signed — personal access tokens

For scripts / non-SSH API access. Tokens are minted and stored; **Bearer-PAT middleware on signed routes is still unfinished** — see [milestones](milestones.md).

- `GET /tokens` — list (`limit`, `offset`, `has_more`); raw secrets are **never** included
- `POST /tokens` — create (body: `name`, `scopes[]`, optional `expires_at`); raw `pat_<prefix>_<secret>` returned **once**. Server stores SHA-256(raw) + indexed prefix only.
- `GET /tokens/{id}` — metadata (`pat_…` or UUID)
- `DELETE /tokens/{id}` — revoke (idempotent)

Scopes: `catalog:read`, `cart:write`, `orders:read`, `orders:write`, `subscriptions:read`, `subscriptions:write`, `profile:read`, `profile:write`.

## Signed — bootstrap

- `GET /view/init` — concurrent hydrate of profile + first catalog page + cart + recent orders. Query: `currency` (default `COP`), `catalog_limit` (default 12, max 100), `orders_limit` (default 5, max 50). Profile is hard-required; failures in `catalog` / `cart` / `orders` drop the section and add a `warnings[]` entry.

## OAuth 2.0 — third-party app registration

- `GET /oauth/clients` — list caller’s apps (`limit`, `offset`, `has_more`); secrets never included
- `POST /oauth/clients` — register (`name`, `redirect_uris[]`, `scopes[]`, optional `confidential` default `true`). Returns `oauth_client` + raw `client_secret` (`ocs_<prefix>_<secret>`) **once**. Absolute `redirect_uri`; `http://` only on loopback (RFC 8252 §7.3).
- `GET /oauth/clients/{id}` — metadata (`oc_…` or UUID)
- `DELETE /oauth/clients/{id}` — revoke (idempotent)

## OAuth 2.0 — Authorization Code grant

No browser session: customer authorize step is **signed** (SSH/HMAC).

- `POST /oauth/authorize` *(signed)* — `client_id` (`oci_…`), `redirect_uri`, `scopes[]`, optional `state`. Returns single-use `oac_<prefix>_<secret>` (10-minute TTL).
- `POST /oauth/token` *(public, `application/x-www-form-urlencoded`)* — `grant_type=authorization_code`, `code`, `redirect_uri`, plus client credentials (HTTP Basic or form). Returns `{ access_token (pat_…), token_type=Bearer, expires_in, scope }`. Errors use RFC 6749 §5.2 shape.

## Signed — checkout & shipping

- `POST /checkout` — pending order, inventory reservation, Stripe Checkout Session (`Idempotency-Key` required). Legacy/explicit entry; `POST /cart/convert` and `POST /orders` share the same engine.
- `GET /shipping/estimate?city=...` — flat shipping estimate by city

## Webhooks

- `POST /webhooks/stripe` (when `STRIPE_WEBHOOK_SECRET` is set; path overridable via `STRIPE_WEBHOOK_PATH`). Handles `checkout.session.completed/expired`, `payment_intent.succeeded/payment_failed`, and `charge.refunded` / `refund.updated`.

## Public — newsletter

Ley 1581 double-opt-in.

- `POST /email` — subscribe (`{"email": "...", "source": "web"}`); consent IP + User-Agent; confirmation email gated by `email_subscribe_confirm`. Idempotent for `CONFIRMED` / `PENDING`. Rate-limited.
- `GET /email/confirm?token=subc_<prefix>_<secret>` — 48-hour TTL, single-use.
- `POST /email/unsubscribe` — `{"email": "..."}`, idempotent.

## Admin

`Authorization: Bearer` token; optional RBAC via `ADMIN_SUPPORT_TOKENS` / `ADMIN_FINANCE_TOKENS`; optional `X-Admin-Actor` for audit.

- Products / prices: `GET|POST|PUT|PATCH|DELETE /admin/products…`, `POST|PUT|DELETE /admin/prices…`, `GET /admin/products/{id}/price-history` (finance+)
- Inventory: `GET /admin/inventory`, `GET /admin/inventory/reservations`, `POST|PUT /admin/inventory`, product image upload `POST /admin/products/{id}/image`
- Orders: list/detail/cancel/reconcile-stripe/status override/refund/refunds list/shipment PATCH + transition/export CSV
- `GET /admin/audit-events`
- Stripe DLQ: `GET /admin/stripe/dead-letters`, `POST /admin/stripe/dead-letters/replay?id=` or `?limit=`
- `GET /docs` — Redoc
- `GET /admin/console` — read-only ops UI (finance+)

See [runbooks](runbooks/) for FAILED shipment follow-up and incident response.
