# Database migrations

Linear, gap-free SQL applied by `services/core-api/internal/migrate` from this directory (`MIGRATIONS_PATH`, default `infra/migrations`).

## Ladder

Files match `^[0-9]{3}_[a-z0-9_]+\.sql$` and must run in order from `001` with **no gaps** and **no duplicate prefixes**. Enforce with:

```bash
bash scripts/check-migration-linearity.sh
```

The migrator creates `schema_migrations` itself — do **not** add that table here.

## Anchors

| Version | What |
|---------|------|
| **005** | `outbox_events` + `outbox_dead_letters` |
| **018** | Flat `shipping_rates` (Bogotá/Medellín/Cali COP) |
| **025** | Base `invoices` |
| **030** | DIAN columns on `invoices` |
| **031** | `email_subscribers` + `email_confirmation_tokens` |
| **032** | `products.weight_grams`, `shipping_rate_tiers` (+ Bogotá seed) |

## Notes

- Prefer plain `CREATE` / `ALTER` (migrations run once). Idempotent forms are reserved for rare re-seed upserts (e.g. Bogotá flat rate in 032).
- `infra/migrations/*.sql` is gitignored for operator workflows; keep a local copy so fresh clones can migrate.
