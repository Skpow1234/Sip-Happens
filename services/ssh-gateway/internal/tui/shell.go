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
	case screenSplash, screenWelcome, screenError:
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
	if banner := m.renderProfileLinkBanner(); banner != "" {
		parts = append(parts, banner)
	}
	if opts.checkout != nil {
		parts = append(parts, m.renderBreadcrumbs(*opts.checkout))
	}
	parts = append(parts, body)
	if m.notice != "" {
		parts = append(parts, m.renderNoticeBar())
	}
	if opts.showPromo {
		w := m.width
		if w < 40 {
			w = 80
		}
		parts = append(parts, stylePromo().Width(w).Render(L.ShellPromo))
	}
	if opts.footer != "" {
		parts = append(parts, m.renderFooter(opts.footer))
	}
	return strings.Join(parts, "\n")
}

// renderProfileLinkBanner surfaces a one-line nudge across the shell when
// the SSH fingerprint resolved to an empty customer record. Returns "" when
// the profile is fleshed out or hasn't been fetched yet (it relies on
// state.profileIncomplete which only flips after a successful /view/init).
func (m Model) renderProfileLinkBanner() string {
	if !m.state.profileIncomplete {
		return ""
	}
	w := m.width
	if w < 40 {
		w = 80
	}
	return styleBanner().Width(w).Render(L.ProfileLinkBanner)
}

func (m Model) renderTopBar(active shellTab) string {
	w := m.width
	if w < 40 {
		w = 80
	}
	cell := w / 4
	if cell < 12 {
		cell = 12
	}

	mk := func(key, label string, tab shellTab, first bool) string {
		activeTab := tab == active
		var text string
		if key != "" {
			text = key + " " + label
		} else {
			text = label
		}
		if tab == tabCart {
			text = fmt.Sprintf("%s %s %s [%d]", key, label, formatMoneyShort("COP", m.state.cartSubtotal()), m.state.cartCount())
		}
		st := styleTopCell(first).Width(cell).Align(lipgloss.Center)
		if key == "" {
			return st.Render(renderShellBrandWordmark())
		}
		if activeTab {
			return st.Render(styleTabLabel(true).Render(text))
		}
		return st.Render(styleTabLabel(false).Render(text))
	}

	cells := []string{
		mk("", L.ShellBrand, tabShop, true),
		mk("s", L.ShellShop, tabShop, false),
		mk("a", L.ShellAccount, tabAccount, false),
		mk("c", L.ShellCart, tabCart, false),
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(colorBorder).
		Render(row)
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
	return lipgloss.NewStyle().Width(w).Background(colorDanger).Foreground(colorText).Render(inner)
}

func (m Model) renderFooter(hints string) string {
	return styleFooter().Render(formatFooterHints(hints))
}

func selectedLine(mark, text string, selected bool) string {
	if selected {
		prefix := mark
		if prefix == "" {
			prefix = ""
		}
		return styleRowSelect().Render(prefix+text) + "\n"
	}
	return styleMuted().Render("  " + text) + "\n"
}

func catalogLine(name string, width int, selected bool) string {
	line := truncate(name, max(8, width-2))
	if selected {
		return styleRowSelect().Width(width).Render(line) + "\n"
	}
	return styleMuted().Render("  " + line) + "\n"
}

func splitHorizontal(leftWidth int, left, right string) string {
	l := lipgloss.NewStyle().Width(leftWidth).Render(left)
	r := lipgloss.NewStyle().Render(right)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, "  ", r)
}
