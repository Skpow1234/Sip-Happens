package tui

import (
	"fmt"
	"strings"
	"time"
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
	b.WriteString(styleSection().Render("~ "+L.CatalogFeatured+" ~") + "\n\n")
	if m.state.catalogSearchEdit {
		b.WriteString(L.CatalogSearchPrompt + " " + m.state.catalogSearchBuf + "█\n")
		return b.String()
	}
	if len(m.state.catalog) == 0 {
		b.WriteString("\n" + L.CatalogNoMatches + "\n")
		return b.String()
	}
	for i, it := range m.state.catalog {
		name := it.Product.Name
		b.WriteString(catalogLine(name, width, i == m.state.catalogCursor))
	}
	b.WriteString("\n\n" + styleSection().Render("~ "+L.CatalogAll+" ~") + "\n")
	page := m.state.catalogQuery.Offset/catalogPageSize + 1
	var meta strings.Builder
	meta.WriteString(styleMuted().Render(L.CatalogPageMeta(page, m.state.catalogQuery.Offset)))
	if m.state.catalogHasMore {
		meta.WriteString(stylePromo().Render("  · n → siguiente"))
	}
	if m.state.catalogQuery.Offset > 0 {
		meta.WriteString(stylePromo().Render("  · p ← anterior"))
	}
	b.WriteString(meta.String() + "\n")
	return b.String()
}

func (m Model) catalogDetailPane() string {
	d := m.state.productDetail
	if m.busy && d == nil {
		return styleMuted().Render(L.LoadingProduct) + "\n"
	}
	if d == nil {
		return styleMuted().Render(L.CatalogSelectHint) + "\n"
	}
	p := d.Product
	var b strings.Builder
	b.WriteString(styleWordmark().Render(p.Name) + "\n\n")
	b.WriteString(styleMuted().Render(fmt.Sprintf("%s  |  %s  |  %s",
		fmt.Sprintf(L.ProductVolume, p.VolumeML), fmt.Sprintf(L.ProductABV, p.ABV), p.Brewery)) + "\n\n")
	b.WriteString(stylePrice().Render(formatMoney(d.Price.Currency, d.Price.AmountMinorUnits)) + "\n\n")
	if desc := strings.TrimSpace(p.Description); desc != "" {
		b.WriteString(styleMuted().Render(desc) + "\n\n")
	}
	if p.IBU != nil {
		b.WriteString(styleMuted().Render(fmt.Sprintf("%s: %d", L.ProductIBU, *p.IBU)) + "\n")
	}
	b.WriteString(styleMuted().Render(fmt.Sprintf("%s: %d", L.ProductAvailable, d.AvailableQty)) + "\n\n")
	b.WriteString(formatProductImageBlock(p.ImageURL))
	b.WriteString("\n\n" + renderProductCTA() + "\n")
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
		{accountSubscriptions, L.AccountSubscriptions},
		{accountAddresses, L.AccountAddresses},
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
	case accountAddresses:
		return m.accountAddressesPane()
	case accountSubscriptions:
		return m.accountSubscriptionsPane()
	default:
		return m.accountOrdersPane()
	}
}

func (m Model) accountAddressesPane() string {
	if m.busy && len(m.state.savedAddresses) == 0 {
		return L.Loading + "\n"
	}
	if len(m.state.savedAddresses) == 0 {
		return L.AccountAddressesEmpty + "\n\n" + L.AccountAddressesHelp + "\n"
	}
	var b strings.Builder
	for _, a := range m.state.savedAddresses {
		b.WriteString(formatAddressLine(a))
		b.WriteByte('\n')
		if detail := strings.TrimSpace(a.AddressLine1); detail != "" {
			b.WriteString("    ")
			b.WriteString(detail)
			b.WriteByte('\n')
		}
		if strings.TrimSpace(a.FullName) != "" {
			b.WriteString("    ")
			b.WriteString(a.FullName)
			if strings.TrimSpace(a.Phone) != "" {
				b.WriteString(" · ")
				b.WriteString(a.Phone)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	b.WriteString(L.AccountAddressesHelp + "\n")
	return b.String()
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

func (m Model) accountSubscriptionsPane() string {
	if m.busy && len(m.state.subscriptions) == 0 {
		return L.Loading + "\n"
	}
	if len(m.state.subscriptions) == 0 {
		return L.AccountSubscriptionsEmpty + "\n"
	}
	var b strings.Builder
	for i, sub := range m.state.subscriptions {
		line := formatSubscriptionLine(sub)
		b.WriteString(selectedLine("", line, i == m.state.subscriptionCursor))
	}
	b.WriteString("\n" + L.AccountSubscriptionsHelp + "\n")
	return b.String()
}

func (m Model) viewSubscriptionConfirm() string {
	d := m.state.subDraft
	interval := formatSubscriptionInterval(d.interval)
	price := formatMoney(d.currency, d.unitAmount)
	body := styleWordmark().Render(L.SubscriptionConfirmTitle) + "\n\n"
	body += L.SubscriptionConfirmProduct(d.productName, price, interval, d.qty) + "\n\n"
	if label := subscriptionAddressLabel(m.state.savedAddresses, d.addressID); label != "" {
		body += L.SubscriptionConfirmAddress(label) + "\n"
	} else if len(m.state.savedAddresses) > 0 {
		body += styleMuted().Render(L.SubscriptionConfirmNoAddress) + "\n"
	} else {
		body += styleMuted().Render(L.SubscriptionConfirmNoAddress) + "\n"
	}
	opts := m.layoutDefaults(tabShop, L.SubscriptionConfirmFooter)
	return m.composeLayout(body, opts)
}

func (m Model) viewSubscriptionStatus() string {
	subID := strings.TrimSpace(m.state.lastSubscriptionID)
	if subID == "" {
		return styleMuted().Render(L.Loading) + "\n"
	}
	var b strings.Builder
	b.WriteString(styleWordmark().Render(L.SubscriptionStatusTitle) + "\n\n")
	b.WriteString("ID: ")
	b.WriteString(subID)
	b.WriteByte('\n')
	if st := strings.TrimSpace(m.state.lastSubscriptionStatus); st != "" {
		b.WriteString("Estado: ")
		b.WriteString(st)
		b.WriteByte('\n')
	}
	for _, sub := range m.state.subscriptions {
		if sub.ID == subID {
			b.WriteString(formatSubscriptionLine(sub))
			b.WriteByte('\n')
			break
		}
	}
	opts := m.layoutDefaults(tabAccount, L.SubscriptionStatusHelp)
	return m.composeLayout(b.String(), opts)
}

func (m Model) viewCartV2() string {
	step := crumbCart
	var inner string
	if len(m.state.cart) == 0 {
		inner = L.CartEmpty + "\n"
	} else {
		var lines []string
		for i, l := range m.state.cart {
			line := fmt.Sprintf("%s x%d  %s", l.Name, l.Qty, formatMoney(l.Currency, l.LineTotal()))
			if i == m.cartCursor() {
				lines = append(lines, styleRowSelect().Render(line))
			} else {
				lines = append(lines, styleMuted().Render("  "+line))
			}
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
	var addrBlock strings.Builder
	if len(m.state.savedAddresses) > 0 {
		addrBlock.WriteString(L.CheckoutSavedAddresses)
		addrBlock.WriteByte('\n')
		for i, a := range m.state.savedAddresses {
			line := formatAddressLine(a)
			if m.state.checkoutPickAddress && i == m.state.addressCursor {
				addrBlock.WriteString(styleRowSelect().Render(line))
			} else {
				addrBlock.WriteString(styleMuted().Render(line))
			}
			addrBlock.WriteByte('\n')
		}
		if m.state.checkoutPickAddress {
			addrBlock.WriteString(styleMuted().Render(L.CheckoutAddressPickHelp))
			addrBlock.WriteByte('\n')
		} else {
			addrBlock.WriteString(styleMuted().Render(L.CheckoutAddressRePick))
			addrBlock.WriteByte('\n')
		}
		addrBlock.WriteByte('\n')
	}
	var extra strings.Builder
	if addrBlock.Len() > 0 {
		extra.WriteString(addrBlock.String())
	}
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
	orderLabel := L.PaymentOrder
	var inner string
	if m.state.paymentKind == paymentKindSubscription {
		orderLabel = L.SubscriptionPaymentOrder
		status := m.state.lastSubscriptionStatus
		if status == "" {
			status = "PENDING"
		}
		inner = fmt.Sprintf("%s %s\n%s %s\n\n%s\n\n%s\n\n%s\n\n%s\n",
			orderLabel, m.state.lastSubscriptionID,
			L.PaymentStatus, status,
			L.PaymentOffTerminal,
			L.PaymentQRHint+"\n"+qr,
			formatSelectableURL(m.state.stripeURL, m.width),
			L.PaymentCopyTip)
	} else {
		inner = fmt.Sprintf("%s %s\n%s %s\n%s%s\n\n%s\n\n%s\n\n%s\n\n%s\n",
			orderLabel, m.state.orderLabel(),
			L.PaymentStatus, m.state.orderStatus,
			expiry, reservation,
			L.PaymentOffTerminal,
			L.PaymentQRHint+"\n"+qr,
			formatSelectableURL(m.state.stripeURL, m.width),
			L.PaymentCopyTip)
	}
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
