package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var welcomeLogoASCII = []string{
	`      * · * · *`,
	`     * ▓▓▓▓▓ *`,
	`    .───────────.`,
	`   /   S I P     \`,
	`  │ ░▒▓█████▓▒░ │`,
	`  │ ░▒▓█████▓▒░ │`,
	`   \___________/`,
}

var welcomeLogoStyles = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(colorFoam),
	lipgloss.NewStyle().Foreground(colorBeerMid).Bold(true),
	lipgloss.NewStyle().Foreground(colorBrand),
	lipgloss.NewStyle().Foreground(colorBeerMid).Bold(true),
	lipgloss.NewStyle().Foreground(colorBeerMid),
	lipgloss.NewStyle().Foreground(colorBeerDeep),
	lipgloss.NewStyle().Foreground(colorBrand),
}

func renderWelcomeLogo() string {
	var b strings.Builder
	for i, line := range welcomeLogoASCII {
		st := welcomeLogoStyles[i]
		if i >= len(welcomeLogoStyles) {
			st = styleBrand()
		}
		b.WriteString(st.Render(line))
		if i < len(welcomeLogoASCII)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderWelcomeWordmark() string {
	return styleWordmark().Render("sip-happens") + styleInputCursor().Render("█")
}

func renderWelcomeTagline() string {
	return styleMuted().Render("cerveza artesanal · Colombia")
}
