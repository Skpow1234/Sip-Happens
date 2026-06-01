package tui

import (
	"strings"
	"testing"
)

func TestRenderBirthDateInput_EmptyUsesPlaceholder(t *testing.T) {
	got := stripANSI(renderBirthDateInput(""))
	if !strings.Contains(got, "____-__-__") {
		t.Fatalf("expected date placeholder, got %q", got)
	}
	if strings.Count(got, "█") != 1 {
		t.Fatalf("expected single cursor rune, got %q", got)
	}
}

func TestRenderBirthDateInput_ValueNotCTA(t *testing.T) {
	got := renderBirthDateInput("1995-06-15")
	// CTA background would wrap the whole string in one gold block; digits must stay plain.
	plain := stripANSI(got)
	if !strings.Contains(plain, "1995-06-15") {
		t.Fatal("missing typed date")
	}
}

func TestPlaceStackedCenter_WithFooterFitsHeight(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.width, m.height = 100, 40
	m.screen = screenWelcome
	main := "MAIN\nLINE"
	footer := "footer line"
	out := m.placeStackedCenter(main, footer)
	if lipglossHeight(out) > 40 {
		t.Fatalf("output taller than terminal: %d", lipglossHeight(out))
	}
	if !strings.Contains(out, "footer line") {
		t.Fatal("missing footer")
	}
}

func lipglossHeight(s string) int {
	return len(strings.Split(strings.TrimRight(s, "\n"), "\n"))
}
