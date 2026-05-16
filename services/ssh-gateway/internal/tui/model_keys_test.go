package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestModel_WelcomeEnter_GoesMenu(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenWelcome
	m.state.birthDateInput = "1995-01-01"

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nm := next.(Model)
	if nm.screen != screenMenu {
		t.Fatalf("screen: got %v want menu", nm.screen)
	}
	if cmd == nil {
		t.Fatal("expected commands after entering menu")
	}
}

func TestModel_MenuKey1_OpensCatalog(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenMenu
	m.busy = false

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	nm := next.(Model)
	if nm.screen != screenCatalog {
		t.Fatalf("screen: got %v want catalog", nm.screen)
	}
	if !nm.busy {
		t.Fatal("expected busy while loading catalog")
	}
	if cmd == nil {
		t.Fatal("expected load catalog command")
	}
}

func TestModel_CatalogEsc_ReturnsMenu(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenCatalog

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := next.(Model)
	if nm.screen != screenMenu {
		t.Fatalf("screen: got %v want menu", nm.screen)
	}
}

func TestModel_MenuHelpToggle(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenMenu

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if next.(Model).screen != screenHelp {
		t.Fatalf("expected help screen")
	}

	next2, _ := next.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next2.(Model).screen != screenMenu {
		t.Fatalf("expected menu after help esc")
	}
}

func TestModel_CatalogCursorMoves(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenCatalog
	m.state.catalog = []api.CatalogItem{
		makeCatalogItem("A"),
		makeCatalogItem("B"),
	}
	m.state.catalogCursor = 0

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if next.(Model).state.catalogCursor != 1 {
		t.Fatalf("expected cursor 1")
	}
}

func makeCatalogItem(sku string) api.CatalogItem {
	var it api.CatalogItem
	it.Product.SKU = sku
	it.Product.Name = sku
	it.AvailableQty = 5
	return it
}

func TestModel_CatalogAddOutOfStock_ShowsError(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenCatalog
	m.state.catalog = []api.CatalogItem{makeCatalogItem("X")}
	m.state.catalog[0].AvailableQty = 0

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	nm := next.(Model)
	if nm.screen != screenError {
		t.Fatalf("expected error screen for out of stock")
	}
}
