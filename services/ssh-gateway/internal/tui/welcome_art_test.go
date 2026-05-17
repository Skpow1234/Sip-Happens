package tui

import (
	"strings"
	"testing"
)

func TestRenderWelcomeLogo_ContainsBrandAndArt(t *testing.T) {
	logo := renderWelcomeLogo()
	if !strings.Contains(logo, "S I P") {
		t.Fatal("logo missing SIP glass lettering")
	}
}

func TestViewWelcome_HasLogoAndForm(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenWelcome
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "sip") || !strings.Contains(plain, "happens") {
		t.Fatalf("welcome missing wordmark: %q", plain)
	}
	if !strings.Contains(view, "fecha de nacimiento") {
		t.Fatalf("welcome missing birth prompt: %q", view)
	}
	if !strings.Contains(view, "cerveza artesanal") {
		t.Fatal("welcome missing tagline")
	}
}
