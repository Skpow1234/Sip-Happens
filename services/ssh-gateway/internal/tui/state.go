package tui

import (
	"fmt"
	"strings"
	"time"

	"sip-happens/services/ssh-gateway/internal/api"
)

type screen int

const (
	screenWelcome screen = iota
	screenMenu
	screenCatalog
	screenProduct
	screenCart
	screenCheckout
	screenConfirm
	screenPayment
	screenOrderStatus
	screenOrderHistory
	screenCatalogFilters
	screenHelp
	screenOrderComplete
	screenError
)

const catalogPageSize = 15

type CartLine struct {
	ProductID string
	SKU       string
	Name      string
	Currency  string
	UnitPrice int64
	Qty       int
	AvailableQty int
}

func (c CartLine) LineTotal() int64 {
	return c.UnitPrice * int64(c.Qty)
}

type checkoutFields struct {
	city, neighborhood, address, phone, legalID, fullName string
	focus                                                 int
}

func (f *checkoutFields) labels() []string {
	return []string{"City", "Neighborhood", "Address", "Phone", "Legal ID", "Full name"}
}

func (f *checkoutFields) values() []string {
	return []string{f.city, f.neighborhood, f.address, f.phone, f.legalID, f.fullName}
}

func (f *checkoutFields) set(idx int, v string) {
	switch idx {
	case 0:
		f.city = v
	case 1:
		f.neighborhood = v
	case 2:
		f.address = v
	case 3:
		f.phone = v
	case 4:
		f.legalID = v
	case 5:
		f.fullName = v
	}
}

func (f *checkoutFields) current() string {
	vals := f.values()
	if f.focus < 0 || f.focus >= len(vals) {
		return ""
	}
	return vals[f.focus]
}

func (f *checkoutFields) setCurrent(v string) {
	f.set(f.focus, v)
}

type AppState struct {
	ageConfirmed    bool
	birthDateInput  string
	birthDate       string // YYYY-MM-DD after age gate
	privacyAccepted bool
	cart            []CartLine
	checkout        checkoutFields

	catalog       []api.CatalogItem
	catalogCursor int
	catalogQuery  api.CatalogQuery
	catalogHasMore bool
	catalogSearchEdit bool
	catalogSearchBuf  string
	catalogFilter     catalogFilterFields
	productSKU    string
	productDetail *api.CatalogDetail

	lastOrderID string
	stripeURL   string
	orderStatus string
	orderExpiresAt *time.Time
	lastOrderTotal int64
	lastOrderCurrency string
	lastShipment   *api.ShipmentView
	clock          time.Time

	orderHistory       []api.OrderView
	orderHistoryCursor int
	// allowedOrderIDs tracks orders created this session or selected from history.
	allowedOrderIDs map[string]struct{}

	idempotencyKey string

	// Shipping estimate for checkout / cart (from GET /shipping/estimate).
	shippingMinor       int64
	shippingCurrency    string
	shippingEstimateErr string
}

func (s *AppState) rememberOrderID(id string) {
	if id == "" {
		return
	}
	if s.allowedOrderIDs == nil {
		s.allowedOrderIDs = map[string]struct{}{}
	}
	s.allowedOrderIDs[id] = struct{}{}
}

func (s *AppState) orderIDAllowed(id string) bool {
	if id == "" {
		return false
	}
	if s.allowedOrderIDs != nil {
		if _, ok := s.allowedOrderIDs[id]; ok {
			return true
		}
	}
	for _, o := range s.orderHistory {
		if o.ID == id {
			return true
		}
	}
	return false
}

func (s *AppState) cartCount() int {
	n := 0
	for _, l := range s.cart {
		n += l.Qty
	}
	return n
}

func (s *AppState) cartSubtotal() int64 {
	var t int64
	for _, l := range s.cart {
		t += l.LineTotal()
	}
	return t
}

func (s *AppState) addToCart(item api.CatalogItem, qty int) {
	if qty <= 0 {
		qty = 1
	}
	pid := item.Product.ID
	for i := range s.cart {
		if s.cart[i].ProductID == pid {
			s.cart[i].Qty += qty
			s.cart[i].AvailableQty = item.AvailableQty
			return
		}
	}
	s.cart = append(s.cart, CartLine{
		ProductID:    pid,
		SKU:          item.Product.SKU,
		Name:         item.Product.Name,
		Currency:     item.Price.Currency,
		UnitPrice:    item.Price.AmountMinorUnits,
		Qty:          qty,
		AvailableQty: item.AvailableQty,
	})
}

func (s *AppState) validateCartStock() error {
	if len(s.cart) == 0 {
		return fmt.Errorf("cart is empty")
	}
	for _, l := range s.cart {
		if l.Qty <= 0 {
			return fmt.Errorf("%s: invalid quantity", l.SKU)
		}
		if l.AvailableQty >= 0 && l.Qty > l.AvailableQty {
			return fmt.Errorf("%s: only %d available (cart has %d)", l.SKU, l.AvailableQty, l.Qty)
		}
	}
	return nil
}

func (s *AppState) setOrderSnapshot(order api.OrderView, shipment *api.ShipmentView) {
	s.orderStatus = order.Status
	s.lastOrderTotal = order.TotalMinorUnits
	if order.Currency != "" {
		s.lastOrderCurrency = order.Currency
	}
	s.orderExpiresAt = order.ExpiresAt
	s.lastShipment = shipment
}

type catalogFilterFields struct {
	style, brewery, abvMin, abvMax string
	focus                          int
}

func (f *catalogFilterFields) labels() []string {
	return []string{"Style", "Brewery", "ABV min %", "ABV max %"}
}

func (f *catalogFilterFields) values() []string {
	return []string{f.style, f.brewery, f.abvMin, f.abvMax}
}

func (f *catalogFilterFields) set(idx int, v string) {
	switch idx {
	case 0:
		f.style = v
	case 1:
		f.brewery = v
	case 2:
		f.abvMin = v
	case 3:
		f.abvMax = v
	}
}

func (f *catalogFilterFields) current() string {
	vals := f.values()
	if f.focus < 0 || f.focus >= len(vals) {
		return ""
	}
	return vals[f.focus]
}

func (f *catalogFilterFields) setCurrent(v string) {
	f.set(f.focus, v)
}

func (s *AppState) resetCatalogPage() {
	s.catalogQuery = api.CatalogQuery{Limit: catalogPageSize, Offset: 0}
	s.catalogHasMore = false
	s.catalogSearchBuf = ""
	s.catalogSearchEdit = false
}

func (s *AppState) catalogFilterSummary() string {
	var parts []string
	if s.catalogQuery.Q != "" {
		parts = append(parts, "q="+s.catalogQuery.Q)
	}
	if s.catalogQuery.Style != "" {
		parts = append(parts, "style="+s.catalogQuery.Style)
	}
	if s.catalogQuery.Brewery != "" {
		parts = append(parts, "brewery="+s.catalogQuery.Brewery)
	}
	if s.catalogQuery.ABVMin != nil {
		parts = append(parts, fmt.Sprintf("abv>=%.1f", *s.catalogQuery.ABVMin))
	}
	if s.catalogQuery.ABVMax != nil {
		parts = append(parts, fmt.Sprintf("abv<=%.1f", *s.catalogQuery.ABVMax))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "  ")
}

func (s *AppState) findCartIndex(productID string) int {
	for i, l := range s.cart {
		if l.ProductID == productID {
			return i
		}
	}
	return -1
}
