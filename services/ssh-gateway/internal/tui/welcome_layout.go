package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const welcomeCompactHeight = 26

func renderBirthDateInput(value string) string {
	var b strings.Builder
	if value == "" {
		b.WriteString(styleMuted().Render("____-__-__"))
	} else {
		b.WriteString(styleText().Render(value))
	}
	b.WriteString(styleInputCursor().Render("█"))
	return b.String()
}

func wrapWelcomeURLBlock(width int, line string) string {
	if width <= 0 {
		return line
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(line)
}

// placeStackedCenter centers content in the terminal. When footer is non-empty,
// main + footer are centered as one block so legal links stay visible above the
// bottom edge.
func (m Model) placeStackedCenter(main, footer string) string {
	w := m.width
	if w <= 0 {
		if footer == "" {
			return main
		}
		return main + "\n\n" + footer
	}

	mainBlock := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(main)
	h := m.height

	if footer == "" {
		if h <= 0 {
			return lipgloss.Place(w, 0, lipgloss.Center, lipgloss.Top, mainBlock)
		}
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, mainBlock)
	}

	footerBlock := wrapWelcomeURLBlock(w, footer)
	combined := mainBlock + "\n\n" + footerBlock
	if h <= 0 {
		return lipgloss.Place(w, 0, lipgloss.Center, lipgloss.Top, combined)
	}

	ch := lipgloss.Height(combined)
	if ch >= h {
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Top, combined)
	}
	topPad := (h - ch) / 2
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Top, strings.Repeat("\n", topPad)+combined)
}

func (m Model) welcomeIsCompact() bool {
	return m.height > 0 && m.height < welcomeCompactHeight
}
