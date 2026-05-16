package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// configureTerminalColors picks a palette safe for SSH clients (PuTTY, cmd, etc.).
// Terminal.shop-style apps target 16-color ANSI by default; 256/truecolor is opt-in.
func configureTerminalColors() {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("SIP_TUI_COLOR"))); v != "" {
		switch v {
		case "truecolor", "24bit", "rgb":
			lipgloss.SetColorProfile(termenv.TrueColor)
			return
		case "256", "ansi256":
			lipgloss.SetColorProfile(termenv.ANSI256)
			return
		case "ansi", "16", "legacy":
			lipgloss.SetColorProfile(termenv.ANSI)
			return
		}
	}
	if os.Getenv("SIP_TUI_TRUECOLOR") == "1" {
		lipgloss.SetColorProfile(termenv.TrueColor)
		return
	}
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		// Respect modern terminals unless forced to legacy via SIP_TUI_COLOR=ansi.
		lipgloss.SetColorProfile(termenv.TrueColor)
		return
	}
	// Default: 16 ANSI colors — works in legacy SSH, Windows Terminal, and most emulators.
	lipgloss.SetColorProfile(termenv.ANSI)
}

func init() {
	configureTerminalColors()
}
