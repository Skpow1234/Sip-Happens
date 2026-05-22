package cart

import (
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
)

// StoredLineMatchesProduct reports whether a persisted cart row refers to the product.
func StoredLineMatchesProduct(line customers.CartLine, p *domain.Product) bool {
	if p == nil {
		return false
	}
	want := catalog.PublicID(p.ID)
	ref := strings.TrimSpace(line.ProductID)
	if ref == want || ref == string(p.ID) {
		return true
	}
	if line.SKU != "" && line.SKU == p.SKU {
		return true
	}
	return false
}

// ApplyItemQty sets or removes one product line. qty 0 removes; qty > 0 upserts.
func ApplyItemQty(stored []customers.CartLine, p *domain.Product, normalized customers.CartLine, qty int) []customers.CartLine {
	if p == nil {
		return stored
	}
	idx := -1
	for i, line := range stored {
		if StoredLineMatchesProduct(line, p) {
			idx = i
			break
		}
	}
	if qty <= 0 {
		if idx < 0 {
			return stored
		}
		return append(stored[:idx], stored[idx+1:]...)
	}
	normalized.Qty = qty
	if idx >= 0 {
		stored[idx] = normalized
		return stored
	}
	return append(stored, normalized)
}
