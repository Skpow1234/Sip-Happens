package tui

import (
	"strings"
	"testing"
)

func TestSpanishCO_WelcomeUsesSpanishCopy(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenWelcome
	view := m.View()
	if !strings.Contains(view, "fecha de nacimiento") {
		t.Fatalf("expected Spanish welcome, got: %q", view)
	}
	if strings.Contains(view, "Enter your birth date") {
		t.Fatal("expected no English welcome copy")
	}
}

func TestSpanishCO_MenuUsesSpanishCopy(t *testing.T) {
	m := NewRootModel(nil, "", "", "")
	m.screen = screenMenu
	view := m.View()
	if !strings.Contains(view, "Menú principal") || !strings.Contains(view, "Ver catálogo") {
		t.Fatalf("expected Spanish menu: %q", view)
	}
}
