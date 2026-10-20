# SSH TUI

Bubble Tea UI served by `ssh-gateway`. Spanish (Colombia) copy lives in `services/ssh-gateway/internal/tui/messages_es_co.go`.

## Flow

1. Brief `sip-happens█` splash  
2. Age confirmation  
3. Single signed `GET /view/init` (profile + cart + featured catalog; falls back to sequential loaders)  
4. Split-pane catalog → cart → delivery breadcrumbs → Stripe checkout URL / QR (mock when `STRIPE_MOCK=1`)

Skeleton customers (no name / email / legal ID) get a one-line yellow banner to complete the profile.

## Shortcuts

| Key | Action |
|-----|--------|
| `s` | Shop |
| `a` | Account |
| `c` | Cart |
| `?` | Help |
| `←` / `→` | Add / remove on catalog |
| `g` | Toggle destacados / catálogo |
| `b` | Suscribirse (recurring on selected product) |
| `enter` | Checkout from cart |
| `/` | Search |
| `f` | Filters (style, brewery, ABV) |
| `n` / `p` | Pages |

**Destacados:** newest active products, 6 per page → `GET /catalog?featured=true`.  
**Catálogo:** full list, 15 per page.

Account (`a`, then `↑/↓` or `tab`): pedidos · suscripciones · direcciones · tokens de API (`n` nuevo, `x` revocar) · FAQ · acerca de.

## Payment screen

- Transient network errors while polling order status recover silently (no popup over the QR).
- Past `expires_at`, QR/URL hide; recovery: `c` — restart from cart.
- Shipment labels (`PACKED`, `HANDED_OVER`, `FAILED`, …) via `ShipmentStatusLabel`.

Product detail (`GET /catalog/{id}`) is cached per session so browsing does not re-hit the network for already-opened products.
