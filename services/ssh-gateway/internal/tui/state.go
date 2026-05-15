package tui

import "sip-happens/services/ssh-gateway/internal/api"

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
	screenError
)

type CartLine struct {
	ProductID string
	SKU       string
	Name      string
	Currency  string
	UnitPrice int64
	Qty       int
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
	ageConfirmed bool
	cart         []CartLine
	checkout     checkoutFields

	catalog       []api.CatalogItem
	catalogCursor int
	productSKU    string
	productDetail *api.CatalogDetail

	lastOrderID string
	stripeURL   string
	orderStatus string

	idempotencyKey string
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
			return
		}
	}
	s.cart = append(s.cart, CartLine{
		ProductID: pid,
		SKU:       item.Product.SKU,
		Name:      item.Product.Name,
		Currency:  item.Price.Currency,
		UnitPrice: item.Price.AmountMinorUnits,
		Qty:       qty,
	})
}

func (s *AppState) findCartIndex(productID string) int {
	for i, l := range s.cart {
		if l.ProductID == productID {
			return i
		}
	}
	return -1
}
