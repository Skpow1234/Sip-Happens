package tui

import (
	"strings"
	"testing"
)

func TestSpanishCO_WelcomeUsesSpanishCopy(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenWelcome
	view := m.View()
	if !strings.Contains(view, "fecha de nacimiento") {
		t.Fatalf("expected Spanish welcome, got: %q", view)
	}
	if strings.Contains(view, "Enter your birth date") {
		t.Fatal("expected no English welcome copy")
	}
}

func TestSpanishCO_ShellUsesSpanishCopy(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.width = 80
	view := m.View()
	if !strings.Contains(view, "tienda") || !strings.Contains(view, "carrito") {
		t.Fatalf("expected Spanish shell tabs: %q", view)
	}
}

func TestSpanishCO_AccountMenuSpanish(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenAccount
	m.width = 80
	if !strings.Contains(m.View(), "historial de pedidos") {
		t.Fatalf("expected Spanish account menu")
	}
	if !strings.Contains(m.View(), "mis suscripciones") {
		t.Fatalf("expected subscriptions in account menu")
	}
}
