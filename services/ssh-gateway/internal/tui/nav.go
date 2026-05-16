package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"sip-happens/services/ssh-gateway/internal/api"
)

func (m *Model) friendlyErr(err error) string {
	if err == nil {
		return ""
	}
	if ae, ok := err.(*api.APIError); ok {
		return ae.FriendlyMessage()
	}
	return err.Error()
}

func (m *Model) setNotice(err error) {
	if err == nil {
		m.notice = ""
		return
	}
	m.notice = m.friendlyErr(err)
}

func (m *Model) clearNotice() {
	m.notice = ""
}

func (m *Model) showErr(err error) {
	m.setNotice(err)
}

func (m *Model) activeTab() shellTab {
	switch m.screen {
	case screenCart, screenCheckout, screenConfirm:
		return tabCart
	case screenAccount, screenOrderStatus, screenHelp:
		if m.screen == screenHelp && m.helpFromAccount {
			return tabAccount
		}
		if m.screen == screenAccount || m.screen == screenOrderStatus {
			return tabAccount
		}
		return tabShop
	case screenPayment, screenOrderComplete:
		return tabCart
	default:
		return tabShop
	}
}

func (m *Model) goShop() (tea.Model, tea.Cmd, bool) {
	m.screen = screenCatalog
	m.clearNotice()
	if len(m.state.catalog) == 0 {
		m.busy = true
		return m, m.loadCatalogCmd(), true
	}
	return m, m.maybeLoadCatalogDetailCmd(), true
}

func (m *Model) goAccount() (tea.Model, tea.Cmd, bool) {
	m.screen = screenAccount
	m.state.accountSection = accountOrders
	m.clearNotice()
	if m.state.accountSection == accountOrders {
		m.busy = true
		return m, m.loadOrderHistoryCmd(), true
	}
	return m, nil, true
}

func (m *Model) goCart() (tea.Model, tea.Cmd, bool) {
	m.screen = screenCart
	m.state.catalogCursor = 0
	m.clearNotice()
	if strings.TrimSpace(m.state.checkout.city) != "" {
		return m, m.loadShippingEstimateCmd(), true
	}
	return m, nil, true
}

func (m *Model) tabKeysAllowed() bool {
	switch m.screen {
	case screenCheckout, screenConfirm, screenWelcome, screenCatalogFilters:
		return false
	case screenCatalog:
		return !m.state.catalogSearchEdit
	default:
		return true
	}
}

func (m *Model) handleGlobalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if !m.usesShell() {
		return m, nil, false
	}
	switch msg.String() {
	case "esc":
		if m.notice != "" {
			m.clearNotice()
			return m, nil, true
		}
	case "?":
		m.helpFromAccount = m.screen == screenAccount
		m.screen = screenHelp
		return m, nil, true
	}
	if !m.tabKeysAllowed() {
		return m, nil, false
	}
	switch msg.String() {
	case "s":
		return m.goShop()
	case "a":
		if m.screen == screenCatalog {
			return m, nil, false
		}
		return m.goAccount()
	case "c":
		return m.goCart()
	}
	return m, nil, false
}

func (m *Model) maybeLoadCatalogDetailCmd() tea.Cmd {
	if len(m.state.catalog) == 0 {
		return nil
	}
	i := m.state.catalogCursor
	if i < 0 || i >= len(m.state.catalog) {
		i = 0
	}
	sku := m.state.catalog[i].Product.SKU
	if sku == "" || sku == m.state.productSKU {
		return nil
	}
	m.state.productSKU = sku
	m.busy = true
	return m.loadDetailCmd(sku)
}

// accountSection identifiers
const (
	accountOrders = iota
	accountFAQ
	accountAbout
)
