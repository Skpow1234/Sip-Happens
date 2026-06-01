package tui

import (
	"strings"
	"testing"
)

func TestTopBarCellWidth_FitsTerminal(t *testing.T) {
	w := 120
	cell := topBarCellWidth(w)
	total := topBarCellCount*cell + topBarBorderChars
	if total > w {
		t.Fatalf("top bar wider than terminal: %d > %d", total, w)
	}
}

func TestFitFooterText_TruncatesOnNarrowWidth(t *testing.T) {
	hints := "↑/↓ productos  ←/→ agregar  b suscribirse  / buscar  f filtros  n/p página  c carrito  ? ayuda"
	got := fitFooterText(hints, 50)
	if len(got) > 50 {
		t.Fatalf("footer too long: %d %q", len(got), got)
	}
	if !strings.Contains(got, "↑/↓") {
		t.Fatal("expected first chunk preserved")
	}
}

func TestCatalogPaneWidths_SumWithinTerminal(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.width = 100
	left, right := m.catalogPaneWidths()
	gap := splitPaneGap + 1
	if left+right+gap > 100 {
		t.Fatalf("panes overflow: %d + %d + %d > 100", left, right, gap)
	}
}
