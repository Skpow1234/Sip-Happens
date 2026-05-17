package tui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sip-happens/pkg/shipping"
	"sip-happens/services/ssh-gateway/internal/api"
)

type Model struct {
	api *api.Client

	privacyPolicyURL        string
	consumerRightsURL       string
	dataProcessingPolicyURL string
	supportContact          string
	dispatchETAPolicy       string

	screen screen
	state  AppState

	notice string
	err    error
	busy   bool
	width  int
	height int

	helpFromAccount bool
}

func NewRootModel(client *api.Client, privacyPolicyURL, consumerRightsURL, dataProcessingPolicyURL, supportContact, dispatchETAPolicy string) Model {
	if strings.TrimSpace(privacyPolicyURL) == "" {
		privacyPolicyURL = DefaultPrivacyPolicyURL
	}
	if strings.TrimSpace(consumerRightsURL) == "" {
		consumerRightsURL = DefaultConsumerRightsURL
	}
	if strings.TrimSpace(dataProcessingPolicyURL) == "" {
		dataProcessingPolicyURL = DefaultDataProcessingPolicyURL
	}
	return Model{
		api:                     client,
		privacyPolicyURL:        privacyPolicyURL,
		consumerRightsURL:       consumerRightsURL,
		dataProcessingPolicyURL: dataProcessingPolicyURL,
		supportContact:          supportContact,
		dispatchETAPolicy:       dispatchETAPolicy,
		screen:                  screenWelcome,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadServerCartCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.busy && msg.String() != "ctrl+c" && msg.String() != "q" {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.screen == screenWelcome {
				return m, tea.Quit
			}
			if m.screen == screenError {
				return m, tea.Quit
			}
			if m.usesShell() {
				return m, tea.Quit
			}
			return m, nil
		}
		return m.updateKey(msg)

	case catalogLoadedMsg:
		m.busy = false
		m.state.catalog = msg.page.Items
		m.state.catalogHasMore = msg.page.HasMore
		if m.state.catalogCursor >= len(m.state.catalog) {
			m.state.catalogCursor = 0
		}
		return m, m.maybeLoadCatalogDetailCmd()

	case detailLoadedMsg:
		m.busy = false
		m.state.productDetail = msg.detail
		return m, nil

	case checkoutDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.showErr(msg.err)
			m.screen = screenConfirm
			return m, nil
		}
		m.state.lastOrderID = msg.result.Order.ID
		m.state.lastOrderDisplayRef = msg.result.Order.DisplayRef
		m.state.rememberOrderID(msg.result.Order.ID)
		m.state.setOrderSnapshot(msg.result.Order, nil)
		if msg.result.Stripe != nil {
			m.state.stripeURL = msg.result.Stripe.URL
		}
		m.state.cart = nil
		m.state.clock = time.Now()
		m.screen = screenPayment
		return m, tea.Batch(paymentTickCmd(), paymentPollCmd())

	case orderLoadedMsg:
		m.busy = false
		if msg.err != nil {
			m.showErr(msg.err)
			return m, nil
		}
		prev := m.state.orderStatus
		m.state.setOrderSnapshot(msg.order, msg.shipment)
		if msg.order.Status == "PAID" && prev != "PAID" {
			m.screen = screenOrderComplete
			return m, nil
		}
		if msg.order.Status == "PAID" || msg.order.Status == "CANCELLED" {
			return m, nil
		}
		if m.screen == screenPayment {
			return m, paymentPollCmd()
		}
		return m, m.pollOrderCmd()

	case paymentTickMsg:
		m.state.clock = msg.now
		if m.screen == screenPayment {
			return m, paymentTickCmd()
		}
		return m, nil

	case orderHistoryLoadedMsg:
		m.busy = false
		m.state.orderHistory = msg.orders
		for _, o := range msg.orders {
			m.state.rememberOrderID(o.ID)
		}
		if m.state.orderHistoryCursor >= len(m.state.orderHistory) {
			m.state.orderHistoryCursor = 0
		}
		return m, nil

	case customerLoadedMsg:
		m.busy = false
		if msg.customer != nil {
			m.prefillCheckout(*msg.customer)
			if msg.customer.BirthDate != "" && m.state.birthDate == "" {
				m.state.birthDate = msg.customer.BirthDate
			}
		}
		if m.screen == screenCheckout {
			return m, m.loadShippingEstimateCmd()
		}
		return m, nil

	case cartLoadedMsg:
		m.busy = false
		m.state.cart = cartLinesFromAPI(msg.items)
		return m, nil

	case shippingEstimateMsg:
		if msg.err != nil {
			m.state.shippingEstimateErr = msg.err.Error()
			m.state.shippingMinor = 0
			m.state.shippingCurrency = ""
			return m, nil
		}
		m.state.shippingEstimateErr = ""
		m.state.shippingMinor = msg.minor
		m.state.shippingCurrency = msg.currency
		return m, nil

	case tickMsg:
		if m.screen == screenOrderStatus && m.state.lastOrderID != "" {
			return m, m.fetchOrderCmd()
		}
		if m.screen == screenPayment && m.state.lastOrderID != "" {
			return m, m.fetchOrderCmd()
		}
		return m, nil

	case errMsg:
		m.busy = false
		m.showErr(msg.err)
		return m, nil
	}

	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if next, cmd, ok := m.handleGlobalKey(msg); ok {
		return next, cmd
	}
	switch m.screen {
	case screenWelcome:
		switch msg.String() {
		case "n", "N":
			return m, tea.Quit
		case "enter":
			if err := validateAdultBirthDate(m.state.birthDateInput); err != nil {
				m.err = err
				m.screen = screenError
				return m, nil
			}
			m.state.birthDate = strings.TrimSpace(m.state.birthDateInput)
			m.state.ageConfirmed = true
			m.state.resetCatalogPage()
			m.screen = screenCatalog
			m.busy = true
			return m, tea.Batch(m.patchBirthDateCmd(), m.loadServerCartCmd(), m.loadCatalogCmd())
		case "backspace":
			m.state.birthDateInput = backspaceBirthDateInput(m.state.birthDateInput)
		default:
			if len(msg.Runes) == 1 && msg.Type == tea.KeyRunes {
				m.state.birthDateInput = appendBirthDateInput(m.state.birthDateInput, msg.Runes[0])
			}
		}

	case screenHelp:
		if msg.String() == "esc" || msg.String() == "?" {
			if m.helpFromAccount {
				next, cmd, _ := m.goAccount()
				return next, cmd
			}
			next, cmd, _ := m.goShop()
			return next, cmd
		}

	case screenOrderComplete:
		switch msg.String() {
		case "esc", "enter", " ":
			next, cmd, _ := m.goShop()
			return next, cmd
		}

	case screenAccount:
		switch msg.String() {
		case "tab":
			if m.state.accountSection < accountAbout {
				m.state.accountSection++
			} else {
				m.state.accountSection = accountOrders
			}
			if m.state.accountSection == accountOrders {
				m.busy = true
				return m, m.loadOrderHistoryCmd()
			}
		case "up", "k":
			if m.state.accountSection == accountOrders && len(m.state.orderHistory) > 0 {
				if m.state.orderHistoryCursor > 0 {
					m.state.orderHistoryCursor--
				}
			} else if m.state.accountSection > accountOrders {
				m.state.accountSection--
			}
		case "down", "j":
			if m.state.accountSection == accountOrders && len(m.state.orderHistory) > 0 {
				if m.state.orderHistoryCursor < len(m.state.orderHistory)-1 {
					m.state.orderHistoryCursor++
				}
			} else if m.state.accountSection < accountAbout {
				m.state.accountSection++
				if m.state.accountSection == accountOrders {
					m.busy = true
					return m, m.loadOrderHistoryCmd()
				}
			}
		case "enter":
			if m.state.accountSection == accountOrders && len(m.state.orderHistory) > 0 {
				o := m.state.orderHistory[m.state.orderHistoryCursor]
				m.state.lastOrderID = o.ID
				m.state.lastOrderDisplayRef = o.DisplayRef
				m.state.rememberOrderID(o.ID)
				m.state.orderStatus = o.Status
				m.screen = screenOrderStatus
				m.busy = true
				return m, tea.Batch(m.fetchOrderCmd(), m.pollTickCmd())
			}
		case "r":
			if m.state.accountSection == accountOrders {
				m.busy = true
				return m, m.loadOrderHistoryCmd()
			}
		}

	case screenCatalog:
		if m.state.catalogSearchEdit {
			return m.updateCatalogSearchKey(msg)
		}
		switch msg.String() {
		case "/":
			m.state.catalogSearchEdit = true
			m.state.catalogSearchBuf = m.state.catalogQuery.Q
		case "f":
			m.state.syncCatalogFilterForm()
			m.state.catalogFilter.focus = 0
			m.screen = screenCatalogFilters
		case "n":
			if m.state.catalogHasMore {
				m.state.catalogQuery.Offset += catalogPageSize
				m.state.catalogCursor = 0
				m.busy = true
				return m, m.loadCatalogCmd()
			}
		case "p":
			if m.state.catalogQuery.Offset > 0 {
				off := m.state.catalogQuery.Offset - catalogPageSize
				if off < 0 {
					off = 0
				}
				m.state.catalogQuery.Offset = off
				m.state.catalogCursor = 0
				m.busy = true
				return m, m.loadCatalogCmd()
			}
		case "up", "k":
			if m.state.catalogCursor > 0 {
				m.state.catalogCursor--
				return m, m.maybeLoadCatalogDetailCmd()
			}
		case "down", "j":
			if m.state.catalogCursor < len(m.state.catalog)-1 {
				m.state.catalogCursor++
				return m, m.maybeLoadCatalogDetailCmd()
			}
		case "r":
			m.busy = true
			return m, m.loadCatalogCmd()
		case "enter":
			return m, m.maybeLoadCatalogDetailCmd()
		case "right", "l":
			return m.catalogCartDelta(1)
		case "left", "h":
			return m.catalogCartDelta(-1)
		}

	case screenCatalogFilters:
		return m.updateCatalogFilterKey(msg)

	case screenCart:
		switch msg.String() {
		case "esc":
			next, cmd, _ := m.goShop()
			return next, cmd
		case "up", "k":
			if m.cartCursor() > 0 {
				m.setCartCursor(m.cartCursor() - 1)
			}
		case "down", "j":
			if m.cartCursor() < len(m.state.cart)-1 {
				m.setCartCursor(m.cartCursor() + 1)
			}
		case "+", "=":
			m.bumpCartQty(1)
			return m, m.saveServerCartCmd()
		case "-", "_":
			m.bumpCartQty(-1)
			return m, m.saveServerCartCmd()
		case "d", "x":
			m.removeCartLine()
			return m, m.saveServerCartCmd()
		case "enter":
			if len(m.state.cart) == 0 {
				return m, nil
			}
			if err := m.state.validateCartStock(); err != nil {
				m.showErr(err)
				return m, nil
			}
			m.state.shippingMinor = 0
			m.state.shippingCurrency = ""
			m.state.shippingEstimateErr = ""
			m.screen = screenCheckout
			m.state.checkout.focus = 0
			m.busy = true
			return m, m.loadCustomerMeCmd()
		}

	case screenCheckout:
		return m.updateCheckoutKey(msg)

	case screenConfirm:
		switch msg.String() {
		case "esc", "n":
			m.screen = screenCheckout
		case "p":
			m.state.privacyAccepted = !m.state.privacyAccepted
		case "y":
			if err := m.state.validateCartStock(); err != nil {
				m.showErr(err)
				return m, nil
			}
			if !m.state.privacyAccepted {
				m.showErr(fmt.Errorf("%s", L.ErrPrivacyRequired))
				return m, nil
			}
			m.busy = true
			return m, m.submitCheckoutCmd()
		}

	case screenPayment:
		switch msg.String() {
		case "o":
			m.screen = screenOrderStatus
			m.busy = true
			return m, tea.Batch(m.fetchOrderCmd(), m.pollTickCmd())
		}

	case screenOrderStatus:
		switch msg.String() {
		case "esc":
			return m, nil
		case "r":
			m.busy = true
			return m, m.fetchOrderCmd()
		}

	case screenError:
		switch msg.String() {
		case "esc":
			m.err = nil
			next, cmd, _ := m.goShop()
			return next, cmd
		}
	}
	return m, nil
}

func (m Model) catalogCartDelta(delta int) (tea.Model, tea.Cmd) {
	if len(m.state.catalog) == 0 {
		return m, nil
	}
	it := m.state.catalog[m.state.catalogCursor]
	pid := it.Product.ID

	if delta > 0 {
		if it.AvailableQty <= 0 {
			m.showErr(fmt.Errorf("%s", L.ErrOutOfStock))
			return m, nil
		}
		inCart := m.state.cartQtyForProduct(pid)
		if it.AvailableQty > 0 && inCart+1 > it.AvailableQty {
			m.showErr(fmt.Errorf("%s", L.ErrCartStock(it.Product.SKU, it.AvailableQty, inCart)))
			return m, nil
		}
		m.state.addToCart(it, 1)
		return m, m.saveServerCartCmd()
	}

	if m.state.cartQtyForProduct(pid) == 0 {
		return m, nil
	}
	m.state.adjustCartQty(pid, -1, it.AvailableQty)
	return m, m.saveServerCartCmd()
}

func (m *Model) cartCursor() int {
	// reuse catalogCursor for cart index to avoid extra field
	return m.state.catalogCursor
}

func (m *Model) setCartCursor(i int) {
	m.state.catalogCursor = i
}

func (m *Model) bumpCartQty(delta int) {
	if len(m.state.cart) == 0 {
		return
	}
	i := m.cartCursor()
	if i < 0 || i >= len(m.state.cart) {
		i = 0
	}
	newQty := m.state.cart[i].Qty + delta
	if newQty <= 0 {
		m.removeCartLine()
		return
	}
	max := m.state.cart[i].AvailableQty
	if max > 0 && newQty > max {
		m.showErr(fmt.Errorf("%s", L.ErrCartLineStock(m.state.cart[i].SKU, max)))
		return
	}
	m.state.cart[i].Qty = newQty
}

func (m *Model) removeCartLine() {
	if len(m.state.cart) == 0 {
		return
	}
	i := m.cartCursor()
	if i < 0 || i >= len(m.state.cart) {
		return
	}
	m.state.cart = append(m.state.cart[:i], m.state.cart[i+1:]...)
	if m.state.catalogCursor >= len(m.state.cart) && len(m.state.cart) > 0 {
		m.state.catalogCursor = len(m.state.cart) - 1
	}
}

func (m Model) updateCheckoutKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.state.checkout
	switch msg.String() {
	case "esc":
		m.screen = screenCart
		return m, m.loadShippingEstimateCmd()
	case "tab":
		f.focus = (f.focus + 1) % len(f.labels())
	case "shift+tab":
		f.focus--
		if f.focus < 0 {
			f.focus = len(f.labels()) - 1
		}
	case "enter":
		if f.focus < len(f.labels())-1 {
			f.focus++
			return m, m.loadShippingEstimateCmd()
		}
		if !m.checkoutValid() {
			m.showErr(fmt.Errorf("%s", L.ErrCheckoutFields))
			return m, nil
		}
		if m.state.idempotencyKey == "" {
			m.state.idempotencyKey = randomHex(16)
		}
		m.screen = screenConfirm
		return m, nil
	case "backspace":
		cur := f.current()
		if len(cur) > 0 {
			f.setCurrent(cur[:len(cur)-1])
		}
	default:
		if len(msg.Runes) == 1 && msg.Type == tea.KeyRunes {
			f.setCurrent(f.current() + string(msg.Runes))
		}
	}
	return m, m.loadShippingEstimateCmd()
}

func (m Model) checkoutValid() bool {
	v := m.state.checkout.values()
	for _, s := range v {
		if strings.TrimSpace(s) == "" {
			return false
		}
	}
	if len(m.state.cart) == 0 {
		return false
	}
	if _, err := shipping.CanonicalCity(m.state.checkout.city); err != nil {
		return false
	}
	return true
}

func (m Model) View() string {
	if m.screen == screenWelcome {
		return m.viewWelcome()
	}
	if m.screen == screenError {
		return m.viewError()
	}
	body := ""
	if m.busy && m.screen == screenCatalog && m.state.productDetail == nil {
		body = m.viewBusy()
	} else {
		switch m.screen {
		case screenCatalog:
			body = m.viewCatalogSplit()
		case screenCatalogFilters:
			body = m.composeLayout(m.viewCatalogFilters(), m.layoutDefaults(tabShop, L.FiltersHelp))
		case screenCart:
			body = m.viewCartV2()
		case screenCheckout:
			body = m.viewCheckoutV2()
		case screenConfirm:
			body = m.viewConfirmV2()
		case screenPayment:
			body = m.viewPaymentV2()
		case screenOrderStatus:
			body = m.composeLayout(m.viewOrderStatus(), m.layoutDefaults(tabAccount, L.StatusHelp))
		case screenAccount:
			body = m.viewAccountShell()
		case screenHelp:
			body = m.composeLayout(m.viewHelp(), m.layoutDefaults(m.activeTab(), L.HelpBack))
		case screenOrderComplete:
			body = m.composeLayout(m.viewOrderComplete(), m.layoutDefaults(tabCart, "esc — tienda"))
		default:
			body = ""
		}
	}
	if body == "" && m.busy {
		body = m.composeLayout(m.viewBusy(), m.layoutDefaults(m.activeTab(), ""))
	}
	return body
}

func (m Model) viewBusy() string {
	return styleBrand().Render(L.Loading) + "\n"
}

func (m Model) viewMenu() string {
	title := lipgloss.NewStyle().Bold(true).Render(L.MenuTitle)
	cart := L.MenuCart(m.state.cartCount())
	if label := m.state.orderLabel(); label != "" {
		cart += fmt.Sprintf("  |  %s %s", L.MenuLastOrder, label)
	}
	help := fmt.Sprintf(L.MenuHelp, m.privacyPolicyURL, m.consumerRightsURL)
	return fmt.Sprintf("%s\n%s\n%s", title, cart, help)
}

func (m Model) viewCatalog() string {
	header := lipgloss.NewStyle().Bold(true).Render(L.CatalogTitle)
	page := m.state.catalogQuery.Offset/catalogPageSize + 1
	meta := L.CatalogPageMeta(page, m.state.catalogQuery.Offset)
	if sum := m.state.catalogFilterSummary(); sum != "" {
		meta += "  |  " + sum
	}
	help := L.CatalogHelp + "\n\n"
	if m.state.catalogSearchEdit {
		return header + "\n\n" + meta + "\n\n" + L.CatalogSearchPrompt + " " + m.state.catalogSearchBuf + "█\n\n" + L.CatalogSearchApply + "\n"
	}
	if len(m.state.catalog) == 0 {
		return header + "\n\n" + meta + "\n\n" + L.CatalogNoMatches + "\n" + help
	}
	var b strings.Builder
	b.WriteString(header + "\n\n" + meta + "\n\n")
	for i, it := range m.state.catalog {
		cur := "  "
		if i == m.state.catalogCursor {
			cur = "> "
		}
		invima := strings.TrimSpace(it.Product.InvimaReg)
		if invima == "" {
			invima = "—"
		}
		line := fmt.Sprintf("%s%-14s  %-18s  %s:%-14s  %s  %s:%d",
			cur, it.Product.SKU, truncate(it.Product.Name, 18), L.ProductInvima, truncate(invima, 14),
			formatMoney(it.Price.Currency, it.Price.AmountMinorUnits), L.CatalogAvail, it.AvailableQty)
		if strings.TrimSpace(it.Product.ImageURL) != "" {
			line += "  [img]"
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n" + help)
	return b.String()
}

func (m Model) viewProduct() string {
	d := m.state.productDetail
	if d == nil {
		return L.LoadingProduct + "\n"
	}
	p := d.Product
	title := lipgloss.NewStyle().Bold(true).Render(p.Name)
	img := formatProductImageBlock(p.ImageURL)
	desc := strings.TrimSpace(p.Description)
	descBlock := ""
	if desc != "" {
		descBlock = L.ProductDescription + ": " + desc + "\n"
	}
	ibuBlock := ""
	if p.IBU != nil {
		ibuBlock = fmt.Sprintf("%s: %d\n", L.ProductIBU, *p.IBU)
	}
	body := fmt.Sprintf(`%s
%s: %s  |  %s  |  %s  |  %s
%s: %s
%s: %s
%s: %d
%s%s%s
%s
`,
		title, L.ProductSKU, p.SKU, p.Brewery, fmt.Sprintf(L.ProductABV, p.ABV), fmt.Sprintf(L.ProductVolume, p.VolumeML),
		L.ProductInvima, p.InvimaReg, L.ProductPrice, formatMoney(d.Price.Currency, d.Price.AmountMinorUnits), L.ProductAvailable, d.AvailableQty,
		descBlock, ibuBlock, img, L.ProductHelp)
	return body
}

func (m Model) viewCart() string {
	header := lipgloss.NewStyle().Bold(true).Render(L.CartTitle)
	if len(m.state.cart) == 0 {
		return header + "\n\n" + L.CartEmpty + "\n\nesc — menú\n"
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	for i, l := range m.state.cart {
		cur := "  "
		if i == m.cartCursor() {
			cur = "> "
		}
		avail := ""
		if l.AvailableQty >= 0 {
			avail = L.CartMaxQty(l.AvailableQty)
		}
		b.WriteString(fmt.Sprintf("%s%-14s x%d  %s%s\n",
			cur, l.SKU, l.Qty, formatMoney(l.Currency, l.LineTotal()), avail))
	}
	b.WriteString(fmt.Sprintf("\n%s %s\n", L.CartSubtotal, formatMoney("COP", m.state.cartSubtotal())))
	if c := strings.TrimSpace(m.state.checkout.city); c != "" {
		if m.state.shippingEstimateErr != "" {
			b.WriteString(L.CartShippingEst(c, m.state.shippingEstimateErr) + "\n")
		} else if m.state.shippingMinor > 0 {
			cur := m.state.shippingCurrency
			if cur == "" {
				cur = "COP"
			}
			b.WriteString(L.CartShippingEstValue(c, formatMoney(cur, m.state.shippingMinor)) + "\n")
		}
	} else {
		b.WriteString(L.CartDeliveryCities + "\n")
	}
	b.WriteString("\n" + L.CartHelp + "\n")
	return b.String()
}

func (m Model) viewCheckout() string {
	f := m.state.checkout
	title := lipgloss.NewStyle().Bold(true).Render(L.CheckoutTitle)
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, label := range f.labels() {
		mark := "  "
		if i == f.focus {
			mark = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-12s %s\n", mark, label+":", f.values()[i]))
	}
	b.WriteString("\n" + L.CheckoutHelp + "\n")
	b.WriteString("\n" + L.CheckoutCitiesHint + "\n")
	if m.state.shippingEstimateErr != "" {
		b.WriteString(L.CheckoutShippingErr(m.state.shippingEstimateErr) + "\n")
	} else if strings.TrimSpace(f.city) == "" {
		b.WriteString(L.CheckoutShippingWait + "\n")
	} else {
		cur := m.state.shippingCurrency
		if cur == "" {
			cur = "COP"
		}
		b.WriteString(L.CheckoutShippingEst(formatMoney(cur, m.state.shippingMinor)) + "\n")
	}
	b.WriteString("\n" + L.CheckoutConsumerRightsLine(m.consumerRightsURL) + "\n")
	return b.String()
}

func (m Model) viewConfirm() string {
	title := lipgloss.NewStyle().Bold(true).Render(L.ConfirmTitle)
	sub := m.state.cartSubtotal()
	cur := "COP"
	if m.state.shippingCurrency != "" {
		cur = m.state.shippingCurrency
	}
	ship := m.state.shippingMinor
	total := sub + ship
	body := fmt.Sprintf(`%s %s
%s %s
%s %s
%s %d

%s

%s

%s

%s
`, L.ConfirmSubtotal, formatMoney("COP", sub), L.ConfirmShipping, formatMoney(cur, ship), L.ConfirmTotal, formatMoney("COP", total), L.ConfirmItems, m.state.cartCount(),
		L.ConfirmPrivacyLine(m.state.privacyAccepted, m.privacyPolicyURL),
		L.ConfirmConsumerRightsLine(m.consumerRightsURL),
		L.ConfirmDataProcessingLine(m.dataProcessingPolicyURL), L.ConfirmHelp)
	return title + "\n\n" + body
}

func (m Model) viewPayment() string {
	title := lipgloss.NewStyle().Bold(true).Render(L.PaymentTitle)
	qr := asciiQR(m.state.stripeURL, m.width)
	now := m.state.clock
	if now.IsZero() {
		now = time.Now()
	}
	expiry := ""
	reservation := ""
	if m.state.orderExpiresAt != nil {
		cd := formatExpiryCountdown(m.state.orderExpiresAt, now)
		if cd != "" {
			expiry = L.PaymentPayWithin(cd)
		}
		reservation = L.PaymentReservationTTL(formatExpiryAt(m.state.orderExpiresAt, now))
	}
	urlBlock := formatSelectableURL(m.state.stripeURL, m.width)
	copyHint := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(L.PaymentCopyTip)
	body := fmt.Sprintf(`%s %s
%s %s
%s%s
%s

%s

%s

%s

%s
`, L.PaymentOrder, m.state.orderLabel(), L.PaymentStatus, m.state.orderStatus, expiry, reservation, L.PaymentQRHint, qr, copyHint, urlBlock, L.PaymentHelp)
	return title + "\n\n" + body
}

func (m Model) viewOrderComplete() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render(L.OrderCompleteTitle)
	cur := m.state.lastOrderCurrency
	if cur == "" {
		cur = "COP"
	}
	body := L.OrderCompleteBody(m.state.orderLabel(), formatMoney(cur, m.state.lastOrderTotal), m.state.orderStatus, m.dispatchETAPolicy, m.supportContact) +
		"\n\n" + L.OrderCompleteConsumerRightsLine(m.consumerRightsURL)
	return title + "\n\n" + body
}

func (m Model) viewHelp() string {
	title := lipgloss.NewStyle().Bold(true).Render(L.HelpTitle)
	return title + "\n\n" + L.HelpBody + "\n" + L.HelpBack + "\n"
}

func (m Model) viewCatalogFilters() string {
	f := m.state.catalogFilter
	title := lipgloss.NewStyle().Bold(true).Render(L.FiltersTitle)
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, label := range f.labels() {
		mark := "  "
		if i == f.focus {
			mark = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-12s %s\n", mark, label+":", f.values()[i]))
	}
	b.WriteString("\n" + L.FiltersHelp + "\n")
	return b.String()
}

func (m Model) viewOrderHistory() string {
	header := lipgloss.NewStyle().Bold(true).Render(L.HistoryTitle)
	help := L.HistoryHelp + "\n\n"
	if len(m.state.orderHistory) == 0 {
		return header + "\n\n" + L.HistoryEmpty + "\n\n" + help
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	for i, o := range m.state.orderHistory {
		cur := "  "
		if i == m.state.orderHistoryCursor {
			cur = "> "
		}
		label := formatOrderLabel(o.ID, o.DisplayRef)
		b.WriteString(fmt.Sprintf("%s%-18s  %-18s  %s\n",
			cur, label, o.Status, formatMoney(o.Currency, o.TotalMinorUnits)))
	}
	b.WriteString("\n" + help)
	return b.String()
}

func (m Model) viewOrderStatus() string {
	title := lipgloss.NewStyle().Bold(true).Render(L.StatusTitle)
	body := fmt.Sprintf("%s %s\n%s %s\n\n", L.StatusOrder, m.state.orderLabel(), L.PaymentStatus, m.state.orderStatus)
	if ship := formatShipmentSummary(m.state.orderStatus, m.state.lastShipment); ship != "" {
		body += ship
	}
	if m.state.orderStatus == "PENDING_PAYMENT" && m.state.orderExpiresAt != nil {
		cd := formatExpiryCountdown(m.state.orderExpiresAt, m.state.clock)
		if cd == "" {
			m.state.clock = time.Now()
			cd = formatExpiryCountdown(m.state.orderExpiresAt, m.state.clock)
		}
		if cd != "" {
			body += L.StatusPaymentExpires(cd)
		}
	}
	body += L.StatusHelp + "\n"
	if m.state.orderStatus == "CANCELLED" {
		body += "\n" + L.StatusCancelled + "\n"
	}
	return title + "\n\n" + body
}

func (m Model) viewError() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render(L.ErrorTitle)
	msg := fmt.Sprintf("%v", m.err)
	if ae, ok := m.err.(*api.APIError); ok {
		msg = ae.FriendlyMessage()
	}
	return fmt.Sprintf("%s\n\n%s\n\n%s\n", title, msg, L.ErrorBack)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// --- commands ---

type catalogLoadedMsg struct{ page *api.CatalogPage }
type detailLoadedMsg struct{ detail *api.CatalogDetail }
type checkoutDoneMsg struct {
	result *api.CheckoutResult
	err    error
}
type orderLoadedMsg struct {
	order    api.OrderView
	shipment *api.ShipmentView
	err      error
}
type paymentTickMsg struct{ now time.Time }
type orderHistoryLoadedMsg struct{ orders []api.OrderView }
type customerLoadedMsg struct{ customer *api.CustomerView }
type cartLoadedMsg struct{ items []api.CartLine }
type shippingEstimateMsg struct {
	minor    int64
	currency string
	err      error
}
type errMsg struct{ err error }

func (m *Model) prefillCheckout(c api.CustomerView) {
	f := &m.state.checkout
	if f.city == "" {
		f.city = c.City
	}
	if f.neighborhood == "" {
		f.neighborhood = c.Neighborhood
	}
	if f.address == "" {
		f.address = c.AddressLine1
	}
	if f.phone == "" {
		f.phone = c.Phone
	}
	if f.legalID == "" {
		f.legalID = c.LegalID
	}
	if f.fullName == "" {
		f.fullName = c.FullName
	}
}

type tickMsg struct{}

func (m Model) loadCatalogCmd() tea.Cmd {
	q := catalogQueryForLoad(m.state)
	return func() tea.Msg {
		page, err := m.api.GetCatalog(context.Background(), q)
		if err != nil {
			return errMsg{err: err}
		}
		return catalogLoadedMsg{page: page}
	}
}

func (m Model) updateCatalogSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state.catalogSearchEdit = false
		m.state.catalogSearchBuf = ""
	case "enter":
		m.state.catalogQuery.Q = strings.TrimSpace(m.state.catalogSearchBuf)
		m.state.catalogQuery.Offset = 0
		m.state.catalogSearchEdit = false
		m.busy = true
		return m, m.loadCatalogCmd()
	case "backspace":
		buf := m.state.catalogSearchBuf
		if len(buf) > 0 {
			m.state.catalogSearchBuf = buf[:len(buf)-1]
		}
	default:
		if len(msg.Runes) == 1 && msg.Type == tea.KeyRunes {
			m.state.catalogSearchBuf += string(msg.Runes)
		}
	}
	return m, nil
}

func (m Model) updateCatalogFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.state.catalogFilter
	switch msg.String() {
	case "esc":
		m.screen = screenCatalog
	case "tab":
		f.focus = (f.focus + 1) % len(f.labels())
	case "shift+tab":
		f.focus--
		if f.focus < 0 {
			f.focus = len(f.labels()) - 1
		}
	case "y":
		if err := m.state.applyCatalogFiltersFromForm(); err != nil {
			m.showErr(err)
			return m, nil
		}
		m.screen = screenCatalog
		m.busy = true
		return m, m.loadCatalogCmd()
	case "enter":
		if f.focus < len(f.labels())-1 {
			f.focus++
			return m, nil
		}
		if err := m.state.applyCatalogFiltersFromForm(); err != nil {
			m.showErr(err)
			return m, nil
		}
		m.screen = screenCatalog
		m.busy = true
		return m, m.loadCatalogCmd()
	case "backspace":
		cur := f.current()
		if len(cur) > 0 {
			f.setCurrent(cur[:len(cur)-1])
		}
	default:
		if len(msg.Runes) == 1 && msg.Type == tea.KeyRunes {
			f.setCurrent(f.current() + string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) loadDetailCmd(sku string) tea.Cmd {
	return func() tea.Msg {
		d, err := m.api.GetCatalogDetail(context.Background(), sku)
		if err != nil {
			return errMsg{err: err}
		}
		return detailLoadedMsg{detail: d}
	}
}

func (m Model) submitCheckoutCmd() tea.Cmd {
	f := m.state.checkout
	cart := append([]CartLine(nil), m.state.cart...)
	client := m.api
	idem := m.state.idempotencyKey
	birthDate := m.state.birthDate
	if idem == "" {
		idem = randomHex(16)
	}
	return func() tea.Msg {
		var items []api.CheckoutItem
		for _, l := range cart {
			items = append(items, api.CheckoutItem{ProductID: l.ProductID, Qty: l.Qty})
		}
		req := api.CheckoutRequest{
			Currency:       "COP",
			City:           f.city,
			Neighborhood:   f.neighborhood,
			AddressLine1:   f.address,
			Phone:          f.phone,
			LegalID:        f.legalID,
			FullName:       f.fullName,
			BirthDate:      birthDate,
			PrivacyConsent: true,
			Items:          items,
		}
		res, err := client.Checkout(context.Background(), idem, req)
		return checkoutDoneMsg{result: res, err: err}
	}
}

func (m Model) fetchOrderCmd() tea.Cmd {
	id := m.state.lastOrderID
	allowed := m.state.orderIDAllowed(id)
	return func() tea.Msg {
		if !allowed {
			return orderLoadedMsg{err: fmt.Errorf("%s", L.ErrOrderNotInSession)}
		}
		res, err := m.api.GetOrder(context.Background(), id)
		if err != nil {
			return orderLoadedMsg{err: err}
		}
		return orderLoadedMsg{order: res.Order, shipment: res.Shipment}
	}
}

func paymentTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return paymentTickMsg{now: t}
	})
}

func paymentPollCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) loadServerCartCmd() tea.Cmd {
	client := m.api
	return func() tea.Msg {
		items, err := client.GetCart(context.Background())
		if err != nil {
			return errMsg{err: err}
		}
		return cartLoadedMsg{items: items}
	}
}

func (m Model) loadShippingEstimateCmd() tea.Cmd {
	city := strings.TrimSpace(m.state.checkout.city)
	client := m.api
	return func() tea.Msg {
		if city == "" {
			return shippingEstimateMsg{currency: "COP"}
		}
		est, err := client.GetShippingEstimate(context.Background(), city, "COP")
		if err != nil {
			return shippingEstimateMsg{err: err}
		}
		return shippingEstimateMsg{
			minor:    est.ShippingMinorUnits,
			currency: est.Currency,
		}
	}
}

func (m Model) saveServerCartCmd() tea.Cmd {
	items := cartLinesToAPI(m.state.cart)
	client := m.api
	return func() tea.Msg {
		if err := client.PutCart(context.Background(), items); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m Model) patchBirthDateCmd() tea.Cmd {
	bd := m.state.birthDate
	client := m.api
	return func() tea.Msg {
		if bd == "" {
			return nil
		}
		_, err := client.PatchCustomerMe(context.Background(), api.PatchCustomerRequest{
			BirthDate: &bd,
		})
		if err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m Model) loadOrderHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		orders, err := m.api.ListOrders(context.Background(), 20)
		if err != nil {
			return errMsg{err: err}
		}
		return orderHistoryLoadedMsg{orders: orders}
	}
}

func (m Model) loadCustomerMeCmd() tea.Cmd {
	return func() tea.Msg {
		c, err := m.api.GetCustomerMe(context.Background())
		if err != nil {
			// Non-fatal: checkout still works with manual entry.
			return customerLoadedMsg{customer: nil}
		}
		return customerLoadedMsg{customer: c}
	}
}

func (m Model) pollTickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) pollOrderCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
