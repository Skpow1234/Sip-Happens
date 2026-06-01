package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// welcomePixelLogo is a fixed-width pixel grid (one terminal cell = one pixel).
// Runes map to palette entries via styleLogoPixel; spaces are transparent.
//
//	░▒▓ foam/beer (white + gray) · sparkle (gray) █/▌/▐ glass (gold)
var welcomePixelLogo = []string{
	"      · · · ·      ",
	"    ░▒▓██████▓▒░    ",
	"   ░▒▓████████▓▒░   ",
	"  ░▒▓██████████▓▒░  ",
	" ██████████████████▌",
	" ██████████████████▌",
	" ████▓▓ S I P ▓▓████",
	" ████▓▓▓▓▓▓▓▓▓▓█████",
	" ████▓▓▓▓▓▓▓▓▓▓█████",
	" ████▓▓▓▓▓▓▓▓▓▓█████",
	" ██████████████████▌",
	"  ░▒▓██████████▓▒░  ",
	"   ░▒▓████████▓▒░   ",
	"    ░▒▓██████▓▒░    ",
	"      · · · ·      ",
}

func styleLogoPixel(r rune) lipgloss.Style {
	switch r {
	case ' ':
		return lipgloss.NewStyle()
	case '·':
		return lipgloss.NewStyle().Foreground(colorMuted)
	case '░':
		return lipgloss.NewStyle().Foreground(colorMuted)
	case '▒':
		return lipgloss.NewStyle().Foreground(colorFoam)
	case '▓':
		return lipgloss.NewStyle().Foreground(colorFoam).Bold(true)
	case 'S', 'I', 'P':
		return lipgloss.NewStyle().Foreground(colorFoam).Bold(true)
	case '█', '▌', '▐':
		return lipgloss.NewStyle().Foreground(colorBrandGold).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(colorText)
	}
}

func renderWelcomeLogo() string {
	lines := make([]string, 0, len(welcomePixelLogo))
	for _, row := range welcomePixelLogo {
		lines = append(lines, renderLogoPixelRow(row))
	}
	return strings.Join(lines, "\n")
}

func renderLogoPixelRow(row string) string {
	var b strings.Builder
	b.Grow(len(row) * 12)
	for _, r := range row {
		st := styleLogoPixel(r)
		if r == ' ' {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(st.Render(string(r)))
	}
	return b.String()
}

func renderWelcomeTagline() string {
	return styleMuted().Render("cerveza artesanal · Colombia")
}
