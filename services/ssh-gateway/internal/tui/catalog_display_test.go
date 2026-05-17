package tui

import (
	"strings"
	"testing"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestViewCatalogSplit_HidesSKUAndInvima(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.width = 100
	it := makeCatalogItem("DEMO-SKU-HIDDEN")
	it.Product.Name = "Cerveza de Prueba 330ml"
	it.Product.InvimaReg = "INVIMA-SECRET-001"
	m.state.catalog = []api.CatalogItem{it}
	m.state.catalogCursor = 0
	m.state.productDetail = &api.CatalogDetail{
		Product: it.Product,
		Price:   it.Price,
	}

	view := m.View()
	plain := stripANSI(view)
	if strings.Contains(plain, "DEMO-SKU-HIDDEN") {
		t.Fatalf("catalog must not show SKU: %q", plain)
	}
	if strings.Contains(plain, "INVIMA") {
		t.Fatalf("catalog must not show INVIMA: %q", plain)
	}
	if !strings.Contains(plain, "Cerveza de Prueba") {
		t.Fatalf("catalog should show product name: %q", plain)
	}
}
