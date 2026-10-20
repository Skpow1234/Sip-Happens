# Runbook: shipment FAILED / reservation release

When an admin marks a shipment `FAILED` via `POST /admin/orders/{id}/shipment/transition` (reason required, audited):

1. Confirm order/shipment state in `GET /admin/orders/{id}`.
2. Check inventory: `GET /admin/inventory` and `GET /admin/inventory/reservations` for the SKUs.
3. If reservations leaked, run inventory reconcile (detect-only first):

```bash
docker compose -f infra/compose.yaml run --rm core-jobs inventory-reconcile
# apply fixes only after review:
INV_RECONCILE_FIX=1 docker compose -f infra/compose.yaml run --rm core-jobs inventory-reconcile
```

4. Refunds do **not** auto-restock — issue `POST /admin/orders/{id}/refund` separately if money must return, then adjust inventory deliberately.
5. Notify the customer (`email_shipping_update` if enabled) and record the ops reason in audit.
