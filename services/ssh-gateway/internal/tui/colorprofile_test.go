package tui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestConfigureTerminalColors_DefaultANSI(t *testing.T) {
	os.Unsetenv("SIP_TUI_COLOR")
	os.Unsetenv("SIP_TUI_TRUECOLOR")
	os.Unsetenv("COLORTERM")
	configureTerminalColors()
	if lipgloss.ColorProfile() != termenv.ANSI {
		t.Fatalf("expected ANSI profile, got %v", lipgloss.ColorProfile())
	}
}

func TestConfigureTerminalColors_TrueColorOptIn(t *testing.T) {
	t.Setenv("SIP_TUI_COLOR", "truecolor")
	configureTerminalColors()
	if lipgloss.ColorProfile() != termenv.TrueColor {
		t.Fatalf("expected TrueColor, got %v", lipgloss.ColorProfile())
	}
	configureTerminalColors() // restore default for other tests
}
