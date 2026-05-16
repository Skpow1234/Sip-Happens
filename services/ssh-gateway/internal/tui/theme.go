package tui

import "github.com/charmbracelet/lipgloss"

// Terminal palette — warm amber/gold shop accents on dark backgrounds.
var (
	colorBrand    = lipgloss.Color("220") // gold
	colorAccent   = lipgloss.Color("214") // amber
	colorOnAccent = lipgloss.Color("235") // near-black on amber
	colorMuted    = lipgloss.Color("241")
	colorDim      = lipgloss.Color("238")
	colorText     = lipgloss.Color("252")
	colorSKU      = lipgloss.Color("39")  // cyan
	colorSpec     = lipgloss.Color("75")  // blue
	colorPrice    = lipgloss.Color("82")  // green
	colorSection  = lipgloss.Color("177") // magenta
	colorLink     = lipgloss.Color("39")
	colorPromo    = lipgloss.Color("214")
	colorWarn     = lipgloss.Color("203")
	colorDanger   = lipgloss.Color("9")
	colorBorder   = lipgloss.Color("220")
)

func styleBrand() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
}

func styleSection() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorSection)
}

func styleMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func styleDim() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorDim)
}

func stylePrice() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorPrice)
}

func styleSpec() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorSpec)
}

func styleLink() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorLink).Underline(true)
}

func styleActiveTab() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorOnAccent).Background(colorAccent)
}

func styleInactiveTab() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorMuted)
}

func styleTopBorder() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(colorBorder)
}

func styleFooter() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(colorDim).
		Foreground(colorMuted)
}

func styleBox() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(0, 1)
}

func styleBoxTitle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
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
	return lipgloss.NewStyle().Foreground(colorPromo).Italic(true)
}

func styleInputCursor() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
}
