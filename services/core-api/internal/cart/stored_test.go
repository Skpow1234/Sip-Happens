package cart

import (
	"testing"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
)

func TestApplyItemQty_upsertAndRemove(t *testing.T) {
	p := &domain.Product{ID: "11111111-1111-1111-1111-111111111111", SKU: "SKU-A", Name: "A"}
	pub := catalog.PublicID(p.ID)
	line := customers.CartLine{ProductID: pub, SKU: "SKU-A", Qty: 1}

	stored := ApplyItemQty(nil, p, line, 2)
	if len(stored) != 1 || stored[0].Qty != 2 {
		t.Fatalf("upsert: %+v", stored)
	}

	stored = ApplyItemQty(stored, p, line, 0)
	if len(stored) != 0 {
		t.Fatalf("remove: %+v", stored)
	}
}
