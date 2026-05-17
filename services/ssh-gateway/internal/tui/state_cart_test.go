package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestAdjustCartQty_RemovesLineAtZero(t *testing.T) {
	var s AppState
	it := api.CatalogItem{}
	it.Product.ID = "p1"
	it.Product.SKU = "SKU-1"
	s.addToCart(it, 1)
	s.adjustCartQty("p1", -1, 5)
	if len(s.cart) != 0 {
		t.Fatalf("expected empty cart, got %d lines", len(s.cart))
	}
}

func TestCatalogCartDelta_AddAndRemove(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.state.catalog = []api.CatalogItem{makeCatalogItem("X")}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	nm := modelFromTea(next)
	if nm.state.cartCount() != 1 {
		t.Fatalf("expected 1 in cart after right, got %d", nm.state.cartCount())
	}

	next2, _ := nm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	nm2 := modelFromTea(next2)
	if nm2.state.cartCount() != 0 {
		t.Fatalf("expected empty cart after left, got %d", nm2.state.cartCount())
	}
}
