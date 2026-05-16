package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) layoutDefaults(tab shellTab, footer string) layoutOpts {
	return layoutOpts{
		tab:       tab,
		footer:    footer,
		showTabs:  true,
		showPromo: true,
	}
}

func (m Model) viewCatalogSplit() string {
	leftW := 34
	if m.width > 0 && m.width/3 > 28 {
		leftW = m.width / 3
	}
	if leftW < 28 {
		leftW = 28
	}
	body := splitHorizontal(leftW, m.catalogListPane(leftW), m.catalogDetailPane())
	opts := m.layoutDefaults(tabShop, L.CatalogFooter)
	return m.composeLayout(body, opts)
}

func (m Model) catalogListPane(width int) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("~ "+L.CatalogFeatured+" ~") + "\n")
	if m.state.catalogSearchEdit {
		b.WriteString(L.CatalogSearchPrompt + " " + m.state.catalogSearchBuf + "█\n")
		return b.String()
	}
	if len(m.state.catalog) == 0 {
		b.WriteString("\n" + L.CatalogNoMatches + "\n")
		return b.String()
	}
	for i, it := range m.state.catalog {
		name := truncate(it.Product.Name, width-6)
		line := fmt.Sprintf("%-14s %s", it.Product.SKU, name)
		b.WriteString(selectedLine("", line, i == m.state.catalogCursor))
	}
	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("~ "+L.CatalogAll+" ~") + "\n")
	page := m.state.catalogQuery.Offset/catalogPageSize + 1
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(
		L.CatalogPageMeta(page, m.state.catalogQuery.Offset)) + "\n")
	return b.String()
}

func (m Model) catalogDetailPane() string {
	d := m.state.productDetail
	if m.busy && d == nil {
		return L.LoadingProduct + "\n"
	}
	if d == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(L.CatalogSelectHint) + "\n"
	}
	p := d.Product
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(p.Name) + "\n")
	b.WriteString(fmt.Sprintf("%s  |  %s  |  %s\n",
		fmt.Sprintf(L.ProductVolume, p.VolumeML), fmt.Sprintf(L.ProductABV, p.ABV), p.Brewery))
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render(
		formatMoney(d.Price.Currency, d.Price.AmountMinorUnits)) + "\n\n")
	if desc := strings.TrimSpace(p.Description); desc != "" {
		b.WriteString(desc + "\n\n")
	}
	if p.IBU != nil {
		b.WriteString(fmt.Sprintf("%s: %d\n", L.ProductIBU, *p.IBU))
	}
	b.WriteString(fmt.Sprintf("%s: %s\n", L.ProductInvima, p.InvimaReg))
	b.WriteString(fmt.Sprintf("%s: %d\n\n", L.ProductAvailable, d.AvailableQty))
	b.WriteString(formatProductImageBlock(p.ImageURL))
	b.WriteString("\n\n" + L.ProductDetailFooter + "\n")
	return b.String()
}

func (m Model) viewAccountShell() string {
	leftW := 22
	body := splitHorizontal(leftW, m.accountMenuPane(), m.accountContentPane())
	opts := m.layoutDefaults(tabAccount, L.AccountFooter)
	return m.composeLayout(body, opts)
}

func (m Model) accountMenuPane() string {
	items := []struct {
		id    int
		label string
	}{
		{accountOrders, L.AccountOrders},
		{accountFAQ, L.AccountFAQ},
		{accountAbout, L.AccountAbout},
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString(selectedLine("", it.label, m.state.accountSection == it.id))
	}
	return b.String()
}

func (m Model) accountContentPane() string {
	switch m.state.accountSection {
	case accountFAQ:
		return L.AccountFAQBody + "\n"
	case accountAbout:
		return fmt.Sprintf(L.AccountAboutBody, m.privacyPolicyURL, m.consumerRightsURL) + "\n"
	default:
		return m.accountOrdersPane()
	}
}

func (m Model) accountOrdersPane() string {
	if m.busy && len(m.state.orderHistory) == 0 {
		return L.Loading + "\n"
	}
	if len(m.state.orderHistory) == 0 {
		return L.HistoryEmpty + "\n"
	}
	var b strings.Builder
	for i, o := range m.state.orderHistory {
		label := formatOrderLabel(o.ID, o.DisplayRef)
		line := fmt.Sprintf("%-18s  %-14s  %s", label, o.Status, formatMoney(o.Currency, o.TotalMinorUnits))
		b.WriteString(selectedLine("", line, i == m.state.orderHistoryCursor))
	}
	b.WriteString("\n" + L.HistoryPaneHelp + "\n")
	return b.String()
}

func (m Model) viewCartV2() string {
	step := crumbCart
	var inner string
	if len(m.state.cart) == 0 {
		inner = L.CartEmpty + "\n"
	} else {
		var lines []string
		for i, l := range m.state.cart {
			cur := "  "
			if i == m.cartCursor() {
				cur = "> "
			}
			line := fmt.Sprintf("%s%s x%d  %s", cur, l.Name, l.Qty, formatMoney(l.Currency, l.LineTotal()))
			if l.SKU != l.Name {
				line = fmt.Sprintf("%s (%s)", line, l.SKU)
			}
			lines = append(lines, box("", strings.TrimSpace(line), clampInt(60, max(40, m.width-6))))
		}
		inner = strings.Join(lines, "\n")
		sub := m.state.cartSubtotal()
		inner += fmt.Sprintf("\n%s %s\n", L.CartSubtotal, formatMoney("COP", sub))
		if c := strings.TrimSpace(m.state.checkout.city); c != "" && m.state.shippingMinor > 0 {
			cur := m.state.shippingCurrency
			if cur == "" {
				cur = "COP"
			}
			inner += L.CartShippingEstValue(c, formatMoney(cur, m.state.shippingMinor)) + "\n"
		}
	}
	body := inner + "\n"
	opts := layoutOpts{
		tab:       tabCart,
		checkout:  &step,
		footer:    L.CartFooter,
		showTabs:  true,
		showPromo: true,
	}
	return m.composeLayout(body, opts)
}

func (m Model) viewCheckoutV2() string {
	step := crumbShipping
	f := m.state.checkout
	labels := f.labels()
	vals := f.values()
	leftCol := ""
	rightCol := ""
	for i, label := range labels {
		markL, markR := "  ", "  "
		if i == f.focus {
			markL, markR = "> ", "> "
		}
		leftCol += fmt.Sprintf("%s%-14s\n", markL, label+":")
		rightCol += fmt.Sprintf("%s%s\n", markR, vals[i])
	}
	cols := splitHorizontal(16, leftCol, rightCol)
	var extra strings.Builder
	extra.WriteString("\n" + L.CheckoutCitiesHint + "\n")
	if m.state.shippingEstimateErr != "" {
		extra.WriteString(L.CheckoutShippingErr(m.state.shippingEstimateErr) + "\n")
	} else if strings.TrimSpace(f.city) != "" {
		cur := m.state.shippingCurrency
		if cur == "" {
			cur = "COP"
		}
		extra.WriteString(L.CheckoutShippingEst(formatMoney(cur, m.state.shippingMinor)) + "\n")
	}
	extra.WriteString("\n" + L.CheckoutConsumerRightsLine(m.consumerRightsURL) + "\n")
	opts := layoutOpts{
		tab:       tabCart,
		checkout:  &step,
		footer:    L.CheckoutFooter,
		showTabs:  true,
		showPromo: true,
	}
	return m.composeLayout(cols+extra.String(), opts)
}

func (m Model) viewConfirmV2() string {
	step := crumbConfirm
	sub := m.state.cartSubtotal()
	cur := "COP"
	if m.state.shippingCurrency != "" {
		cur = m.state.shippingCurrency
	}
	ship := m.state.shippingMinor
	total := sub + ship
	inner := fmt.Sprintf("%s %s\n%s %s\n%s %s\n%s %d\n\n%s\n\n%s\n\n%s\n",
		L.ConfirmSubtotal, formatMoney("COP", sub),
		L.ConfirmShipping, formatMoney(cur, ship),
		L.ConfirmTotal, formatMoney("COP", total),
		L.ConfirmItems, m.state.cartCount(),
		L.ConfirmPrivacyLine(m.state.privacyAccepted, m.privacyPolicyURL),
		L.ConfirmConsumerRightsLine(m.consumerRightsURL),
		L.ConfirmDataProcessingLine(m.dataProcessingPolicyURL))
	opts := layoutOpts{
		tab:       tabCart,
		checkout:  &step,
		footer:    L.ConfirmFooter,
		showTabs:  true,
		showPromo: true,
	}
	return m.composeLayout(inner, opts)
}

func (m Model) viewPaymentV2() string {
	step := crumbPayment
	qr := asciiQR(m.state.stripeURL, m.width)
	now := m.state.clock
	if now.IsZero() {
		now = time.Now()
	}
	expiry := ""
	reservation := ""
	if m.state.orderExpiresAt != nil {
		if cd := formatExpiryCountdown(m.state.orderExpiresAt, now); cd != "" {
			expiry = L.PaymentPayWithin(cd)
		}
		reservation = L.PaymentReservationTTL(formatExpiryAt(m.state.orderExpiresAt, now))
	}
	inner := fmt.Sprintf("%s %s\n%s %s\n%s%s\n\n%s\n\n%s\n\n%s\n\n%s\n",
		L.PaymentOrder, m.state.orderLabel(),
		L.PaymentStatus, m.state.orderStatus,
		expiry, reservation,
		L.PaymentOffTerminal,
		L.PaymentQRHint+"\n"+qr,
		formatSelectableURL(m.state.stripeURL, m.width),
		L.PaymentCopyTip)
	opts := layoutOpts{
		tab:       tabCart,
		checkout:  &step,
		footer:    L.PaymentFooter,
		showTabs:  true,
		showPromo: true,
	}
	return m.composeLayout(inner, opts)
}

func clampInt(hi, v int) int {
	if v > hi {
		return hi
	}
	if v < 1 {
		return 1
	}
	return v
}
