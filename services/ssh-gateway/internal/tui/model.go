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
	return nil
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
		m.state.catalog = msg.items
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
		m.state.orderStatus = msg.result.Order.Status
		if msg.result.Stripe != nil {
			m.state.stripeURL = msg.result.Stripe.URL
		}
		m.state.cart = nil
		m.screen = screenPayment
		return m, nil

	case orderLoadedMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			m.screen = screenError
			return m, nil
		}
		m.state.orderStatus = msg.order.Status
		if msg.order.Status == "PAID" || msg.order.Status == "CANCELLED" {
			return m, nil
		}
		return m, m.pollOrderCmd()

	case tickMsg:
		if m.screen == screenOrderStatus && m.state.lastOrderID != "" {
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
		case "y", "Y":
			m.state.ageConfirmed = true
			m.screen = screenMenu
		case "n", "N":
			return m, tea.Quit
		}

	case screenMenu:
		switch msg.String() {
		case "1", "c":
			m.screen = screenCatalog
			m.busy = true
			return m, m.loadCatalogCmd()
		case "2":
			m.state.catalogCursor = 0
			m.screen = screenCart
		case "3", "o":
			if m.state.lastOrderID == "" {
				m.err = fmt.Errorf("no recent order in this session")
				m.screen = screenError
				return m, nil
			}
			m.screen = screenOrderStatus
			m.busy = true
			return m, tea.Batch(m.fetchOrderCmd(), m.pollTickCmd())
		case "q":
			return m, tea.Quit
		}

	case screenCatalog:
		switch msg.String() {
		case "esc":
			m.screen = screenMenu
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
			m.state.addToCart(m.state.catalog[m.state.catalogCursor], 1)
		case "b":
			m.screen = screenCart
		}

	case screenProduct:
		switch msg.String() {
		case "esc":
			m.screen = screenCatalog
		case "a":
			if m.state.productDetail != nil {
				m.state.addToCart(m.state.productDetail.AsCatalogItem(), 1)
			}
		case "b":
			m.screen = screenCart
		}

	case screenCart:
		switch msg.String() {
		case "esc":
			m.screen = screenMenu
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
		case "-", "_":
			m.bumpCartQty(-1)
		case "d", "x":
			m.removeCartLine()
		case "c":
			if len(m.state.cart) == 0 {
				return m, nil
			}
			m.screen = screenCheckout
			m.state.checkout.focus = 0
		}

	case screenCheckout:
		return m.updateCheckoutKey(msg)

	case screenConfirm:
		switch msg.String() {
		case "esc", "n":
			m.screen = screenCheckout
		case "y":
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
	m.state.cart[i].Qty += delta
	if m.state.cart[i].Qty <= 0 {
		m.removeCartLine()
	}
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
			return m, nil
		}
		if !m.checkoutValid() {
			m.err = fmt.Errorf("fill all delivery fields before continuing")
			m.screen = screenError
			return m, nil
		}
		if m.state.idempotencyKey == "" {
			m.state.idempotencyKey = randomHex(16)
		}
		m.screen = screenConfirm
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

func (m Model) checkoutValid() bool {
	v := m.state.checkout.values()
	for _, s := range v {
		if strings.TrimSpace(s) == "" {
			return false
		}
	}
	return len(m.state.cart) > 0
}

func (m Model) View() string {
	if m.busy {
		return m.viewBusy()
	}
	switch m.screen {
	case screenWelcome:
		return m.viewWelcome()
	case screenMenu:
		return m.viewMenu()
	case screenCatalog:
		return m.viewCatalog()
	case screenProduct:
		return m.viewProduct()
	case screenCart:
		return m.viewCart()
	case screenCheckout:
		return m.viewCheckout()
	case screenConfirm:
		return m.viewConfirm()
	case screenPayment:
		return m.viewPayment()
	case screenOrderStatus:
		return m.viewOrderStatus()
	case screenError:
		return m.viewError()
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
	body := "Are you 18 or older?\n\n  y — yes, enter the shop\n  n — no, exit\n"
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
  q — Quit
`
	return fmt.Sprintf("%s\n%s\n%s", title, cart, help)
}

func (m Model) viewCatalog() string {
	header := lipgloss.NewStyle().Bold(true).Render("Catalog")
	help := "↑/↓ move  enter detail  a add  b cart  r refresh  esc menu\n\n"
	if len(m.state.catalog) == 0 {
		return header + "\n\n(Empty catalog — press r to refresh)\n" + help
	}
	var b strings.Builder
	b.WriteString(header + "\n\n")
	for i, it := range m.state.catalog {
		cur := "  "
		if i == m.state.catalogCursor {
			cur = "> "
		}
		line := fmt.Sprintf("%s%-14s  %-22s  %s\n",
			cur, it.Product.SKU, truncate(it.Product.Name, 22),
			formatMoney(it.Price.Currency, it.Price.AmountMinorUnits))
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

a — add to cart   b — cart   esc — back
`,
		title, p.SKU, p.Brewery, p.ABV, p.VolumeML, p.InvimaReg,
		formatMoney(d.Price.Currency, d.Price.AmountMinorUnits))
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
		b.WriteString(fmt.Sprintf("%s%-14s x%d  %s\n",
			cur, l.SKU, l.Qty, formatMoney(l.Currency, l.LineTotal())))
	}
	b.WriteString(fmt.Sprintf("\nSubtotal: %s\n\n", formatMoney("COP", m.state.cartSubtotal())))
	b.WriteString("+/- qty  d remove  c checkout  esc menu\n")
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
	return b.String()
}

func (m Model) viewConfirm() string {
	title := lipgloss.NewStyle().Bold(true).Render("Confirm checkout")
	sub := formatMoney("COP", m.state.cartSubtotal())
	body := fmt.Sprintf("Total: %s\nItems: %d\n\ny — place order   n — back\n", sub, m.state.cartCount())
	return title + "\n\n" + body
}

func (m Model) viewPayment() string {
	title := lipgloss.NewStyle().Bold(true).Render("Pay off-terminal")
	body := fmt.Sprintf(`Order: %s
Status: %s

Open this URL in your browser to pay (cards are not entered in SSH):

%s

o — refresh order status   esc — menu
`, m.state.lastOrderID, m.state.orderStatus, m.state.stripeURL)
	return title + "\n\n" + body
}

func (m Model) viewOrderStatus() string {
	title := lipgloss.NewStyle().Bold(true).Render("Order status")
	body := fmt.Sprintf("Order: %s\nStatus: %s\n\nr refresh  esc menu\n", m.state.lastOrderID, m.state.orderStatus)
	if m.state.orderStatus == "PAID" {
		body += "\nPayment received. Thank you!\n"
	}
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

type catalogLoadedMsg struct{ items []api.CatalogItem }
type detailLoadedMsg struct{ detail *api.CatalogDetail }
type checkoutDoneMsg struct {
	result *api.CheckoutResult
	err    error
}
type orderLoadedMsg struct {
	order api.OrderView
	err   error
}
type errMsg struct{ err error }
type tickMsg struct{}

func (m Model) loadCatalogCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.api.GetCatalog(context.Background())
		if err != nil {
			return errMsg{err: err}
		}
		return catalogLoadedMsg{items: items}
	}
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
	if idem == "" {
		idem = randomHex(16)
	}
	return func() tea.Msg {
		var items []api.CheckoutItem
		for _, l := range cart {
			items = append(items, api.CheckoutItem{ProductID: l.ProductID, Qty: l.Qty})
		}
		req := api.CheckoutRequest{
			Currency:     "COP",
			City:         f.city,
			Neighborhood: f.neighborhood,
			AddressLine1: f.address,
			Phone:        f.phone,
			LegalID:      f.legalID,
			FullName:     f.fullName,
			Items:        items,
		}
		res, err := client.Checkout(context.Background(), idem, req)
		return checkoutDoneMsg{result: res, err: err}
	}
}

func (m Model) fetchOrderCmd() tea.Cmd {
	id := m.state.lastOrderID
	return func() tea.Msg {
		res, err := m.api.GetOrder(context.Background(), id)
		if err != nil {
			return orderLoadedMsg{err: err}
		}
		return orderLoadedMsg{order: res.Order}
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
