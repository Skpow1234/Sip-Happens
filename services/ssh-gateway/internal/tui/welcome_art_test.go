package tui

import (
	"strings"
	"testing"
)

func TestRenderWelcomeLogo_PixelArt(t *testing.T) {
	logo := renderWelcomeLogo()
	plain := stripANSI(logo)
	lines := strings.Split(strings.TrimSpace(plain), "\n")
	if len(lines) < 10 {
		t.Fatalf("expected tall pixel logo, got %d lines", len(lines))
	}
	if !strings.Contains(plain, "S I P") {
		t.Fatal("logo missing SIP lettering in beer")
	}
	if !strings.Contains(plain, "█") {
		t.Fatal("logo missing solid pixel blocks")
	}
	// Foam + beer shading characters should appear in the grid.
	if !strings.Contains(plain, "░") || !strings.Contains(plain, "▓") {
		t.Fatal("logo missing foam/beer pixel shading")
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
