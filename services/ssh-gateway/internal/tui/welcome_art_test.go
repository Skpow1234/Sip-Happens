package tui

import (
	"strings"
	"testing"
)

func TestRenderWelcomeLogo_ContainsBrandAndArt(t *testing.T) {
	logo := renderWelcomeLogo()
	if !strings.Contains(logo, "Sip-Happens") {
		t.Fatal("logo missing wordmark")
	}
	if !strings.Contains(logo, "S I P") {
		t.Fatal("logo missing SIP glass lettering")
	}
}

func TestViewWelcome_HasLogoAndForm(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenWelcome
	view := m.View()
	if !strings.Contains(view, "Sip-Happens") {
		t.Fatalf("welcome missing logo wordmark: %q", view)
	}
	if !strings.Contains(view, "fecha de nacimiento") {
		t.Fatalf("welcome missing birth prompt: %q", view)
	}
	if !strings.Contains(view, "cerveza artesanal") {
		t.Fatal("welcome missing tagline")
	}
}
