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

	screen screen
	state  AppState

	err    error
	busy   bool
	width  int
	height int
}

func NewRootModel(client *api.Client) Model {
	return Model{
		api:    client,
		screen: screenWelcome,
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
		case "ctrl+c", "q":
			if m.screen == screenWelcome || m.screen == screenMenu {
				return m, tea.Quit
			}
			if m.screen == screenError {
				return m, tea.Quit
			}
			m.screen = screenMenu
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
		return m, nil

	case detailLoadedMsg:
		m.busy = false
		m.state.productDetail = msg.detail
		m.screen = screenProduct
		return m, nil

	case checkoutDoneMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		m.state.lastOrderID = msg.result.Order.ID
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
			m.err = msg.err
			m.screen = screenError
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
		m.err = msg.err
		m.screen = screenError
		return m, nil
	}

	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.screen = screenMenu
			m.busy = true
			return m, tea.Batch(m.patchBirthDateCmd(), m.loadServerCartCmd())
		case "backspace":
			if len(m.state.birthDateInput) > 0 {
				m.state.birthDateInput = m.state.birthDateInput[:len(m.state.birthDateInput)-1]
			}
		default:
			if len(msg.Runes) == 1 && msg.Type == tea.KeyRunes {
				m.state.birthDateInput += string(msg.Runes)
			}
		}

	case screenMenu:
		switch msg.String() {
		case "1", "c":
			m.state.resetCatalogPage()
			m.screen = screenCatalog
			m.busy = true
			return m, m.loadCatalogCmd()
		case "2":
			m.state.catalogCursor = 0
			m.screen = screenCart
			if strings.TrimSpace(m.state.checkout.city) != "" {
				return m, m.loadShippingEstimateCmd()
			}
		case "3", "o":
			if m.state.lastOrderID == "" {
				m.err = fmt.Errorf("no recent order in this session")
				m.screen = screenError
				return m, nil
			}
			if !m.state.orderIDAllowed(m.state.lastOrderID) {
				m.err = fmt.Errorf("order is not available in this session")
				m.screen = screenError
				return m, nil
			}
			m.screen = screenOrderStatus
			m.busy = true
			return m, tea.Batch(m.fetchOrderCmd(), m.pollTickCmd())
		case "4":
			m.screen = screenOrderHistory
			m.state.orderHistoryCursor = 0
			m.busy = true
			return m, m.loadOrderHistoryCmd()
		case "?", "h":
			m.screen = screenHelp
		case "q":
			return m, tea.Quit
		}

	case screenHelp:
		if msg.String() == "esc" || msg.String() == "?" {
			m.screen = screenMenu
		}

	case screenOrderComplete:
		switch msg.String() {
		case "esc", "enter", " ":
			m.screen = screenMenu
		}

	case screenOrderHistory:
		switch msg.String() {
		case "esc":
			m.screen = screenMenu
		case "up", "k":
			if m.state.orderHistoryCursor > 0 {
				m.state.orderHistoryCursor--
			}
		case "down", "j":
			if m.state.orderHistoryCursor < len(m.state.orderHistory)-1 {
				m.state.orderHistoryCursor++
			}
		case "r":
			m.busy = true
			return m, m.loadOrderHistoryCmd()
		case "enter":
			if len(m.state.orderHistory) == 0 {
				return m, nil
			}
			o := m.state.orderHistory[m.state.orderHistoryCursor]
			m.state.lastOrderID = o.ID
			m.state.rememberOrderID(o.ID)
			m.state.orderStatus = o.Status
			m.screen = screenOrderStatus
			m.busy = true
			return m, tea.Batch(m.fetchOrderCmd(), m.pollTickCmd())
		}

	case screenCatalog:
		if m.state.catalogSearchEdit {
			return m.updateCatalogSearchKey(msg)
		}
		switch msg.String() {
		case "esc":
			m.screen = screenMenu
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
			}
		case "down", "j":
			if m.state.catalogCursor < len(m.state.catalog)-1 {
				m.state.catalogCursor++
			}
		case "r":
			m.busy = true
			return m, m.loadCatalogCmd()
		case "enter":
			if len(m.state.catalog) == 0 {
				return m, nil
			}
			sku := m.state.catalog[m.state.catalogCursor].Product.SKU
			m.state.productSKU = sku
			m.busy = true
			return m, m.loadDetailCmd(sku)
		case "a":
			if len(m.state.catalog) == 0 {
				return m, nil
			}
			it := m.state.catalog[m.state.catalogCursor]
			if it.AvailableQty <= 0 {
				m.err = fmt.Errorf("out of stock")
				m.screen = screenError
				return m, nil
			}
			m.state.addToCart(it, 1)
			return m, m.saveServerCartCmd()
		case "b":
			m.screen = screenCart
			return m, m.saveServerCartCmd()
		}

	case screenCatalogFilters:
		return m.updateCatalogFilterKey(msg)

	case screenProduct:
		switch msg.String() {
		case "esc":
			m.screen = screenCatalog
		case "a":
			if m.state.productDetail != nil {
				it := m.state.productDetail.AsCatalogItem()
				if it.AvailableQty <= 0 {
					m.err = fmt.Errorf("out of stock")
					m.screen = screenError
					return m, nil
				}
				m.state.addToCart(it, 1)
				return m, m.saveServerCartCmd()
			}
		case "b":
			m.screen = screenCart
			return m, m.saveServerCartCmd()
		}

	case screenCart:
		switch msg.String() {
		case "esc":
			m.screen = screenMenu
			return m, m.saveServerCartCmd()
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
			if m.screen != screenError {
				return m, m.saveServerCartCmd()
			}
		case "-", "_":
			m.bumpCartQty(-1)
			if m.screen != screenError {
				return m, m.saveServerCartCmd()
			}
		case "d", "x":
			m.removeCartLine()
			return m, m.saveServerCartCmd()
		case "c":
			if len(m.state.cart) == 0 {
				return m, nil
			}
			if err := m.state.validateCartStock(); err != nil {
				m.err = err
				m.screen = screenError
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
				m.err = err
				m.screen = screenError
				return m, nil
			}
			if !m.state.privacyAccepted {
				m.err = fmt.Errorf("accept the privacy policy (press p) before continuing")
				m.screen = screenError
				return m, nil
			}
			m.busy = true
			return m, m.submitCheckoutCmd()
		}

	case screenPayment:
		switch msg.String() {
		case "esc":
			m.screen = screenMenu
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
			m.screen = screenMenu
		}
	}
	return m, nil
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
		m.err = fmt.Errorf("%s: only %d available", m.state.cart[i].SKU, max)
		m.screen = screenError
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
			m.err = fmt.Errorf("fill all delivery fields before continuing (city must be Bogotá, Medellín, or Cali)")
			m.screen = screenError
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
	if m.busy {
		return m.withFooter(m.viewBusy())
	}
	switch m.screen {
	case screenWelcome:
		return m.viewWelcome()
	case screenMenu:
		return m.withFooter(m.viewMenu())
	case screenCatalog:
		return m.withFooter(m.viewCatalog())
	case screenCatalogFilters:
		return m.withFooter(m.viewCatalogFilters())
	case screenProduct:
		return m.withFooter(m.viewProduct())
	case screenCart:
		return m.withFooter(m.viewCart())
	case screenCheckout:
		return m.withFooter(m.viewCheckout())
	case screenConfirm:
		return m.withFooter(m.viewConfirm())
	case screenPayment:
		return m.withFooter(m.viewPayment())
	case screenOrderStatus:
		return m.withFooter(m.viewOrderStatus())
	case screenOrderHistory:
		return m.withFooter(m.viewOrderHistory())
	case screenHelp:
		return m.withFooter(m.viewHelp())
	case screenOrderComplete:
		return m.withFooter(m.viewOrderComplete())
	case screenError:
		return m.withFooter(m.viewError())
	default:
		return ""
	}
}

func (m Model) viewBusy() string {
	return lipgloss.NewStyle().Bold(true).Render("Loading…") + "\n"
}

func (m Model) viewWelcome() string {
	title := lipgloss.NewStyle().Bold(true).Render("Sip-Happens")
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Prohibited sale of alcohol to minors.")
	body := fmt.Sprintf(`Enter your birth date (YYYY-MM-DD):

  %s█

enter — continue   n — exit

Privacy: %s
`, m.state.birthDateInput, privacyPolicyURL)
	return fmt.Sprintf("%s\n%s\n\n%s", title, warn, body)
}

func (m Model) viewMenu() string {
	title := lipgloss.NewStyle().Bold(true).Render("Main menu")
	cart := fmt.Sprintf("Cart: %d item(s)", m.state.cartCount())
	if m.state.lastOrderID != "" {
		cart += fmt.Sprintf("  |  Last order: %s", m.state.lastOrderID)
	}
	help := `
  1 — Browse catalog
  2 — View cart
  3 — Check last order status
  4 — Order history
  ? — Help
  q — Quit
`
	return fmt.Sprintf("%s\n%s\n%s", title, cart, help)
}

func (m Model) viewCatalog() string {
	header := lipgloss.NewStyle().Bold(true).Render("Catalog")
	page := m.state.catalogQuery.Offset/catalogPageSize + 1
	meta := fmt.Sprintf("page %d  offset %d", page, m.state.catalogQuery.Offset)
	if sum := m.state.catalogFilterSummary(); sum != "" {
		meta += "  |  " + sum
	}
	help := "↑/↓  enter  a add  b cart  / search  f filters  n/p page  r refresh  esc menu\n\n"
	if m.state.catalogSearchEdit {
		return header + "\n\n" + meta + "\n\nSearch: " + m.state.catalogSearchBuf + "█\n\nenter apply  esc cancel\n"
	}
	if len(m.state.catalog) == 0 {
		return header + "\n\n" + meta + "\n\n(No matches — adjust filters or press r)\n" + help
	}
	var b strings.Builder
	b.WriteString(header + "\n\n" + meta + "\n\n")
	for i, it := range m.state.catalog {
		cur := "  "
		if i == m.state.catalogCursor {
			cur = "> "
		}
		line := fmt.Sprintf("%s%-14s  %-22s  %s  avail:%d\n",
			cur, it.Product.SKU, truncate(it.Product.Name, 22),
			formatMoney(it.Price.Currency, it.Price.AmountMinorUnits), it.AvailableQty)
		b.WriteString(line)
	}
	b.WriteString("\n" + help)
	return b.String()
}

func (m Model) viewProduct() string {
	d := m.state.productDetail
	if d == nil {
		return "Loading product…\n"
	}
	p := d.Product
	title := lipgloss.NewStyle().Bold(true).Render(p.Name)
	body := fmt.Sprintf(`%s
SKU: %s  |  %s  |  %.1f%% ABV  |  %d ml
INVIMA: %s
Price: %s
Available: %d

a — add to cart   b — cart   esc — back
`,
		title, p.SKU, p.Brewery, p.ABV, p.VolumeML, p.InvimaReg,
		formatMoney(d.Price.Currency, d.Price.AmountMinorUnits), d.AvailableQty)
	return body
}

func (m Model) viewCart() string {
	header := lipgloss.NewStyle().Bold(true).Render("Cart")
	if len(m.state.cart) == 0 {
		return header + "\n\n(Empty)\n\nesc — menu\n"
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
			avail = fmt.Sprintf("  max %d", l.AvailableQty)
		}
		b.WriteString(fmt.Sprintf("%s%-14s x%d  %s%s\n",
			cur, l.SKU, l.Qty, formatMoney(l.Currency, l.LineTotal()), avail))
	}
	b.WriteString(fmt.Sprintf("\nSubtotal: %s\n", formatMoney("COP", m.state.cartSubtotal())))
	if c := strings.TrimSpace(m.state.checkout.city); c != "" {
		if m.state.shippingEstimateErr != "" {
			b.WriteString(fmt.Sprintf("Shipping (est.): —  (%s)\n", m.state.shippingEstimateErr))
		} else if m.state.shippingMinor > 0 {
			cur := m.state.shippingCurrency
			if cur == "" {
				cur = "COP"
			}
			b.WriteString(fmt.Sprintf("Shipping (est., %s): %s\n", c, formatMoney(cur, m.state.shippingMinor)))
		}
	} else {
		b.WriteString("Delivery cities: Bogotá, Medellín, Cali — estimate at checkout.\n")
	}
	b.WriteString("\n+/- qty  d remove  c checkout  esc menu\n")
	return b.String()
}

func (m Model) viewCheckout() string {
	f := m.state.checkout
	title := lipgloss.NewStyle().Bold(true).Render("Delivery details")
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, label := range f.labels() {
		mark := "  "
		if i == f.focus {
			mark = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-12s %s\n", mark, label+":", f.values()[i]))
	}
	b.WriteString("\ntab / enter — next field  esc — back to cart\n")
	b.WriteString("\nDelivery cities: Bogotá, Medellín, or Cali.\n")
	if m.state.shippingEstimateErr != "" {
		b.WriteString(fmt.Sprintf("Shipping estimate: —  (%s)\n", m.state.shippingEstimateErr))
	} else if strings.TrimSpace(f.city) == "" {
		b.WriteString("Shipping estimate: (enter city)\n")
	} else {
		cur := m.state.shippingCurrency
		if cur == "" {
			cur = "COP"
		}
		b.WriteString(fmt.Sprintf("Shipping estimate: %s\n", formatMoney(cur, m.state.shippingMinor)))
	}
	return b.String()
}

func (m Model) viewConfirm() string {
	title := lipgloss.NewStyle().Bold(true).Render("Confirm checkout")
	sub := m.state.cartSubtotal()
	cur := "COP"
	if m.state.shippingCurrency != "" {
		cur = m.state.shippingCurrency
	}
	ship := m.state.shippingMinor
	total := sub + ship
	priv := "[ ]"
	if m.state.privacyAccepted {
		priv = "[x]"
	}
	body := fmt.Sprintf(`Subtotal: %s
Shipping (est.): %s
Total: %s
Items: %d

%s I accept the privacy policy (%s)

p — toggle accept   y — place order   n — back
`, formatMoney("COP", sub), formatMoney(cur, ship), formatMoney("COP", total), m.state.cartCount(), priv, privacyPolicyURL)
	return title + "\n\n" + body
}

func (m Model) viewPayment() string {
	title := lipgloss.NewStyle().Bold(true).Render("Pay off-terminal")
	qr := asciiQR(m.state.stripeURL, m.width)
	now := m.state.clock
	if now.IsZero() {
		now = time.Now()
	}
	expiry := ""
	if m.state.orderExpiresAt != nil {
		cd := formatExpiryCountdown(m.state.orderExpiresAt, now)
		if cd != "" {
			expiry = fmt.Sprintf("Pay within: %s\n\n", cd)
		}
	}
	urlBlock := formatSelectableURL(m.state.stripeURL, m.width)
	copyHint := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(
		"Tip: select the URL lines below (mouse drag or terminal copy) and paste into your browser.")
	body := fmt.Sprintf(`Order: %s
Status: %s
%sScan the QR code or open this URL in your browser (cards are not entered in SSH):

%s

%s

%s

o — refresh order status   esc — menu
`, m.state.lastOrderID, m.state.orderStatus, expiry, qr, copyHint, urlBlock)
	return title + "\n\n" + body
}

func (m Model) viewOrderComplete() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")).Render("Payment received")
	cur := m.state.lastOrderCurrency
	if cur == "" {
		cur = "COP"
	}
	body := fmt.Sprintf(`Thank you! Your order is confirmed.

Order: %s
Total: %s
Status: %s

We will prepare your shipment and update status when it ships.
Check order status anytime from the main menu (3 or 4).

enter / esc — back to menu
`, m.state.lastOrderID, formatMoney(cur, m.state.lastOrderTotal), m.state.orderStatus)
	return title + "\n\n" + body
}

func (m Model) viewHelp() string {
	title := lipgloss.NewStyle().Bold(true).Render("Help")
	return title + "\n\n" + helpText + "\nesc — back to menu\n"
}

func (m Model) viewCatalogFilters() string {
	f := m.state.catalogFilter
	title := lipgloss.NewStyle().Bold(true).Render("Catalog filters")
	var b strings.Builder
	b.WriteString(title + "\n\n")
	for i, label := range f.labels() {
		mark := "  "
		if i == f.focus {
			mark = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-12s %s\n", mark, label+":", f.values()[i]))
	}
	b.WriteString("\ntab / enter — next  esc — back without saving  y — apply\n")
	return b.String()
}

func (m Model) viewOrderHistory() string {
	header := lipgloss.NewStyle().Bold(true).Render("Order history")
	help := "↑/↓ move  enter view  r refresh  esc menu\n\n"
	if len(m.state.orderHistory) == 0 {
		return header + "\n\n(No orders yet)\n\n" + help
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	for i, o := range m.state.orderHistory {
		cur := "  "
		if i == m.state.orderHistoryCursor {
			cur = "> "
		}
		shortID := o.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		b.WriteString(fmt.Sprintf("%s%s  %-18s  %s\n",
			cur, shortID, o.Status, formatMoney(o.Currency, o.TotalMinorUnits)))
	}
	b.WriteString("\n" + help)
	return b.String()
}

func (m Model) viewOrderStatus() string {
	title := lipgloss.NewStyle().Bold(true).Render("Order status")
	body := fmt.Sprintf("Order: %s\nStatus: %s\n\n", m.state.lastOrderID, m.state.orderStatus)
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
			body += fmt.Sprintf("Payment expires in: %s\n\n", cd)
		}
	}
	body += "r refresh  esc menu\n"
	if m.state.orderStatus == "CANCELLED" {
		body += "\nOrder was cancelled or expired.\n"
	}
	return title + "\n\n" + body
}

func (m Model) viewError() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("Error")
	msg := fmt.Sprintf("%v", m.err)
	if ae, ok := m.err.(*api.APIError); ok {
		msg = ae.FriendlyMessage()
	}
	return fmt.Sprintf("%s\n\n%s\n\nesc — menu   q — quit\n", title, msg)
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
			m.err = err
			m.screen = screenError
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
			m.err = err
			m.screen = screenError
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
			return orderLoadedMsg{err: fmt.Errorf("order is not available in this session")}
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
