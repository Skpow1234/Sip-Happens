package orders

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

func skuMapFromItems(items []domain.OrderItem) map[domain.ProductID]string {
	m := make(map[domain.ProductID]string, len(items))
	for _, it := range items {
		m[it.ProductID] = it.SKU
	}
	return m
}

func (s *service) recordInventoryReserved(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID, sku string, qty int) {
	if s.audit == nil || tx == nil {
		return
	}
	_ = s.audit.InsertMap(ctx, tx, "order", string(orderID), "orders", "inventory.reserved", map[string]any{
		"product_id": string(productID),
		"sku":        sku,
		"qty":        qty,
	})
}

func (s *service) recordInventoryReleased(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID, sku string, qty int, reason string) {
	if s.audit == nil || tx == nil {
		return
	}
	_ = s.audit.InsertMap(ctx, tx, "order", string(orderID), "orders", "inventory.released", map[string]any{
		"product_id": string(productID),
		"sku":        sku,
		"qty":        qty,
		"reason":     reason,
	})
}
