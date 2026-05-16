package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// welcomeLogoASCII is original byte art: foam, glass, amber fill, and wordmark.
var welcomeLogoASCII = []string{
	`      * · * · *`,
	`     * ▓▓▓▓▓ *`,
	`    .───────────.`,
	`   /   S I P     \`,
	`  │ ░▒▓█████▓▒░ │`,
	`  │ ░▒▓█████▓▒░ │`,
	`   \___________/`,
	`      ║     ║`,
	`     ═╧═════╧═`,
	`   ◆ Sip-Happens ◆`,
}

// welcomeLogoStyles assigns a color pass per line (foam → glass → brand).
var welcomeLogoStyles = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(lipgloss.Color("252")), // foam stars
	lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
	lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Italic(true),
}

func renderWelcomeLogo() string {
	var b strings.Builder
	n := len(welcomeLogoASCII)
	for i, line := range welcomeLogoASCII {
		st := welcomeLogoStyles[i]
		if i >= len(welcomeLogoStyles) {
			st = styleBrand()
		}
		b.WriteString(st.Render(line))
		if i < n-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderWelcomeTagline() string {
	return lipgloss.NewStyle().
		Foreground(colorSpec).
		Italic(true).
		Render("cerveza artesanal · Colombia")
}

func renderWelcomeFrame(inner string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Render(inner)
}
