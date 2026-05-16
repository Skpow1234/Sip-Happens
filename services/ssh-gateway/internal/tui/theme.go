package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette: Terminal.shop-style layout with a Sip-Happens gold/copper shop theme
// (not the reference orange — gold selection, violet key hints, grey chrome).
var (
	colorText       = lipgloss.Color("15")  // primary white
	colorMuted      = lipgloss.Color("245") // labels, inactive list
	colorDim        = lipgloss.Color("240") // borders, separators
	colorAccent     = lipgloss.Color("179") // selection / CTA fill (gold)
	colorOnAccent   = lipgloss.Color("16")  // text on gold
	colorBrand      = lipgloss.Color("220") // wordmark, prices
	colorHint       = lipgloss.Color("135") // enter / action hints (violet)
	colorSpec       = lipgloss.Color("245")
	colorLink       = lipgloss.Color("39")
	colorWarn       = lipgloss.Color("203")
	colorDanger     = lipgloss.Color("9")
	colorBorder     = lipgloss.Color("238")
	colorFoam       = lipgloss.Color("252")
	colorBeerMid    = lipgloss.Color("214")
	colorBeerDeep   = lipgloss.Color("208")
)

func styleBrand() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
}

func styleWordmark() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorText)
}

func styleSection() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func styleMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func styleDim() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorDim)
}

func styleText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorText)
}

func stylePrice() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
}

func styleSpec() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorSpec)
}

func styleLink() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorLink).Underline(true)
}

func styleTabLabel(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Bold(true).Foreground(colorText)
	}
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func styleRowSelect() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorOnAccent).Background(colorAccent)
}

func styleCTA() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorOnAccent).Background(colorAccent).Padding(0, 1)
}

func styleHintKey() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorHint)
}

func styleFooterKey() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorText)
}

func styleTopCell(first bool) lipgloss.Style {
	st := lipgloss.NewStyle().Padding(0, 1)
	if !first {
		st = st.Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(colorBorder)
	}
	return st
}

func styleFooter() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorBorder).
		Foreground(colorMuted)
}

func styleBox() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)
}

func styleBoxTitle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorText)
}

func styleCrumbActive() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
}

func styleCrumbInactive() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorDim)
}

func styleCrumbSep() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorDim)
}

func stylePromo() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted).Align(lipgloss.Center)
}

func styleInputCursor() lipgloss.Style {
	return styleCTA()
}

func styleWelcomeFrame() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2)
}

// formatFooterHints styles shortcut footers like Terminal.shop (bold key, grey label).
func formatFooterHints(hints string) string {
	chunks := splitFooterChunks(hints)
	if len(chunks) == 0 {
		return styleMuted().Render(hints)
	}
	var parts []string
	for _, chunk := range chunks {
		parts = append(parts, formatFooterChunk(chunk))
	}
	return strings.Join(parts, "  ")
}

func splitFooterChunks(hints string) []string {
	var out []string
	for _, c := range strings.Split(hints, "  ") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	return out
}

func formatFooterChunk(chunk string) string {
	i := strings.IndexByte(chunk, ' ')
	if i <= 0 {
		return styleMuted().Render(chunk)
	}
	key, label := chunk[:i], strings.TrimSpace(chunk[i:])
	if label == "" {
		return styleFooterKey().Render(key)
	}
	return styleFooterKey().Render(key) + " " + styleMuted().Render(label)
}

func renderProductCTA(addKey, addLabel, enterKey, enterLabel string) string {
	return styleFooterKey().Render(addKey) + " " +
		styleCTA().Render(addLabel) + "  " +
		styleHintKey().Render(enterKey) + " " +
		styleMuted().Render(enterLabel)
}
