package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sip-happens/services/ssh-gateway/internal/api"
)

func modelFromTea(v tea.Model) Model {
	switch x := v.(type) {
	case Model:
		return x
	case *Model:
		return *x
	default:
		panic("unexpected model type")
	}
}

func TestModel_WelcomeEnter_GoesBootstrap(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenWelcome
	m.state.birthDateInput = "1995-01-01"

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nm := modelFromTea(next)
	if nm.screen != screenBootstrap {
		t.Fatalf("screen: got %v want bootstrap loader", nm.screen)
	}
	if cmd == nil {
		t.Fatal("expected commands after age confirmation")
	}
}

func TestModel_GlobalS_OpensShop(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCart

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	nm := modelFromTea(next)
	if nm.screen != screenCatalog {
		t.Fatalf("expected catalog, got %v", nm.screen)
	}
}

func TestModel_CatalogEsc_DismissesNotice(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.notice = "test error"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := modelFromTea(next)
	if nm.notice != "" {
		t.Fatalf("expected notice cleared")
	}
	if nm.screen != screenCatalog {
		t.Fatalf("expected stay on catalog")
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if modelFromTea(next).screen != screenHelp {
		t.Fatalf("expected help screen")
	}

	next2, _ := modelFromTea(next).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if modelFromTea(next2).screen != screenCatalog {
		t.Fatalf("expected catalog after help esc")
	}
}

func TestModel_CatalogCursorMoves(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.state.catalog = []api.CatalogItem{
		makeCatalogItem("A"),
		makeCatalogItem("B"),
	}
	m.state.catalogCursor = 0

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if modelFromTea(next).state.catalogCursor != 1 {
		t.Fatalf("expected cursor 1")
	}
}

func makeCatalogItem(sku string) api.CatalogItem {
	var it api.CatalogItem
	it.Product.SKU = sku
	it.Product.Name = "Demo " + sku
	it.AvailableQty = 5
	return it
}

func TestModel_CatalogAddOutOfStock_ShowsNotice(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.state.catalog = []api.CatalogItem{makeCatalogItem("X")}
	m.state.catalog[0].AvailableQty = 0

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	nm := modelFromTea(next)
	if nm.notice == "" {
		t.Fatalf("expected notice for out of stock")
	}
	if nm.screen != screenCatalog {
		t.Fatalf("expected stay on catalog")
	}
}
