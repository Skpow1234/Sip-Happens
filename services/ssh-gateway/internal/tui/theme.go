package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Andes nocturno — 16-color ANSI: white + gray chrome, gold accent fills, red alerts.
// Matches Terminal.shop structure: white brand in header, accent pill selection + primary CTA.
var (
	colorText      = lipgloss.Color("15") // primary copy
	colorMuted     = lipgloss.Color("8")  // hints, borders, beer shading
	colorDim       = lipgloss.Color("8")
	colorAccent    = lipgloss.Color("11") // gold — selection, CTAs, links, brand
	colorOnAccent  = lipgloss.Color("0")  // black on gold fills
	colorBrandGold = colorAccent
	colorWarn      = lipgloss.Color("9")
	colorDanger    = lipgloss.Color("9")
	colorBorder    = lipgloss.Color("8")
	colorFoam      = lipgloss.Color("15")
)

func styleBrand() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold)
}

// renderShellBrandWordmark is gold — splash, welcome, loaders.
func renderShellBrandWordmark() string {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold).Render("sip-happens")
}

// renderTopBarBrand is white — shell header matches Terminal.shop “terminal”.
func renderTopBarBrand() string {
	return lipgloss.NewStyle().Bold(true).Foreground(colorText).Render("sip-happens")
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

func styleText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorText)
}

func stylePrice() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold)
}

func styleLink() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorBrandGold).Underline(true)
}

func styleTabLabel(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Bold(true).Foreground(colorText)
	}
	return lipgloss.NewStyle().Foreground(colorMuted)
}

// styleRowSelect — full-width accent bar, black label (Terminal.shop list selection).
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

// styleBrandCursor is the solid block on splash/bootstrap (gold fill, like Terminal.shop).
func styleBrandCursor() lipgloss.Style {
	return styleCTA()
}

// styleInputCursor is the typing caret in forms — gold glyph only, no CTA background.
func styleInputCursor() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrandGold)
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

// renderProductCTA — one accent pill + key hint; secondary action in muted text.
func renderProductCTA() string {
	return styleCTA().Render("agregar") + " " + styleText().Render("→") + "  " +
		styleMuted().Render("b suscribirse")
}
