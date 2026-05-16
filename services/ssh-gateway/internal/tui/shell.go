package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type shellTab int

const (
	tabShop shellTab = iota
	tabAccount
	tabCart
)

type checkoutCrumb int

const (
	crumbCart checkoutCrumb = iota
	crumbShipping
	crumbPayment
	crumbConfirm
)

type layoutOpts struct {
	tab          shellTab
	checkout     *checkoutCrumb
	footer       string
	showTabs     bool
	showPromo    bool
}

func (m Model) usesShell() bool {
	switch m.screen {
	case screenWelcome, screenError:
		return false
	default:
		return true
	}
}

func (m Model) composeLayout(body string, opts layoutOpts) string {
	var parts []string
	if opts.showTabs {
		parts = append(parts, m.renderTopBar(opts.tab))
	}
	if opts.checkout != nil {
		parts = append(parts, m.renderBreadcrumbs(*opts.checkout))
	}
	parts = append(parts, body)
	if m.notice != "" {
		parts = append(parts, m.renderNoticeBar())
	}
	if opts.showPromo {
		parts = append(parts, stylePromo().Align(lipgloss.Center).Width(m.width).Render(L.ShellPromo))
	}
	if opts.footer != "" {
		parts = append(parts, m.renderFooter(opts.footer))
	}
	return strings.Join(parts, "\n")
}

func (m Model) renderTopBar(active shellTab) string {
	w := m.width
	if w < 40 {
		w = 80
	}
	cell := w / 4
	mk := func(key, label string, tab shellTab) string {
		text := key + " " + label
		if tab == tabCart {
			text = fmt.Sprintf("%s %s %s [%d]", key, label, formatMoneyShort("COP", m.state.cartSubtotal()), m.state.cartCount())
		}
		if tab == active {
			return styleActiveTab().Width(cell).Align(lipgloss.Center).Render(text)
		}
		return styleInactiveTab().Width(cell).Align(lipgloss.Center).Render(text)
	}
	brand := styleBrand().Width(cell).Align(lipgloss.Center).Render(L.ShellBrand)
	cells := []string{
		brand,
		mk("s", L.ShellShop, tabShop),
		mk("a", L.ShellAccount, tabAccount),
		mk("c", L.ShellCart, tabCart),
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return styleTopBorder().Render(row)
}

func formatMoneyShort(currency string, minor int64) string {
	if currency == "" {
		currency = "COP"
	}
	return formatMoney(currency, minor)
}

func (m Model) renderBreadcrumbs(step checkoutCrumb) string {
	parts := []struct {
		label string
		step  checkoutCrumb
	}{
		{L.CrumbCart, crumbCart},
		{L.CrumbShipping, crumbShipping},
		{L.CrumbPayment, crumbPayment},
		{L.CrumbConfirm, crumbConfirm},
	}
	var cells []string
	for _, p := range parts {
		if p.step == step {
			cells = append(cells, styleCrumbActive().Render(p.label))
		} else {
			cells = append(cells, styleCrumbInactive().Render(p.label))
		}
	}
	return strings.Join(cells, styleCrumbSep().Render(" / "))
}

func (m Model) renderNoticeBar() string {
	w := m.width
	if w < 20 {
		w = 80
	}
	msg := m.notice
	if len(msg) > w-6 {
		msg = msg[:w-9] + "…"
	}
	inner := msg + strings.Repeat(" ", max(0, w-len(msg)-4)) + "esc"
	return lipgloss.NewStyle().Width(w).Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Render(inner)
}

func (m Model) renderFooter(hints string) string {
	return styleFooter().Render(hints)
}

func selectedLine(mark, text string, selected bool) string {
	if selected {
		prefix := mark
		if prefix == "" {
			prefix = "› "
		}
		return styleActiveTab().Render(prefix + text) + "\n"
	}
	return styleDim().Render("  " + text) + "\n"
}

func catalogLine(sku, name string, selected bool) string {
	skuPart := lipgloss.NewStyle().Foreground(colorSKU).Width(14).Render(truncate(sku, 14))
	namePart := lipgloss.NewStyle().Foreground(colorText).Render(truncate(name, 40))
	line := skuPart + " " + namePart
	if selected {
		return styleActiveTab().Render("› "+line) + "\n"
	}
	return "  " + line + "\n"
}

func box(title, inner string, width int) string {
	if width < 10 {
		width = 40
	}
	titleRendered := ""
	if title != "" {
		titleRendered = styleBoxTitle().Render(title) + "\n"
	}
	return styleBox().Width(width).Render(titleRendered + inner)
}

func splitHorizontal(leftWidth int, left, right string) string {
	l := lipgloss.NewStyle().Width(leftWidth).Render(left)
	r := lipgloss.NewStyle().Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, " ", r)
}
