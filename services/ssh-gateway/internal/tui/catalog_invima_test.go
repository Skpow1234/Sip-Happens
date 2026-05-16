package tui

import (
	"strings"
	"testing"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestViewCatalog_ShowsInvimaOnEachLine(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenCatalog
	it := makeCatalogItem("DEMO-SKU")
	it.Product.InvimaReg = "INVIMA-DEMO-001"
	m.state.catalog = []api.CatalogItem{it}

	view := m.viewCatalog()
	if !strings.Contains(view, "INVIMA:") || !strings.Contains(view, "INVIMA-DEMO") {
		t.Fatalf("catalog view missing INVIMA label: %q", view)
	}
}
