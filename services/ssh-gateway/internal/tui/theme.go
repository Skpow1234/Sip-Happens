package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 16-color ANSI palette — Terminal.shop-style layout, Sip-Happens colors (cyan + gold, no orange/violet).
var (
	colorText       = lipgloss.Color("15")
	colorMuted      = lipgloss.Color("8")
	colorDim          = lipgloss.Color("8")
	colorAccent       = lipgloss.Color("6")  // cyan — list selection / CTA fill
	colorOnAccent     = lipgloss.Color("15") // white on cyan
	colorBrandGold    = lipgloss.Color("11") // prices, accent word
	colorBrandCyan    = lipgloss.Color("14") // brand word, links
	colorSpec         = lipgloss.Color("8")
	colorLink         = lipgloss.Color("14")
	colorWarn         = lipgloss.Color("9")
	colorDanger       = lipgloss.Color("9")
	colorBorder       = lipgloss.Color("8")
	colorFoam         = lipgloss.Color("15")
	colorBeerMid      = lipgloss.Color("14")
	colorBeerDeep     = lipgloss.Color("6")
)

func styleBrand() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold)
}

func renderShellBrandWordmark() string {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandCyan).Render("sip") +
		lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold).Render("-happens")
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
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold)
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
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold)
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

func styleErr() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorWarn)
}

func styleBanner() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorBrandGold).Italic(true)
}

func renderProductCTA() string {
	return styleFooterKey().Render("←") + " " + styleMuted().Render("quitar") + "  " +
		styleFooterKey().Render("→") + " " + styleCTA().Render("agregar") + "  " +
		styleFooterKey().Render("b") + " " + styleCTA().Render("suscribirse")
}
