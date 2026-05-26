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
	screenAccount
	screenSubscriptionConfirm
	screenSubscriptionStatus
	screenTokenCreate
	screenTokenShow
	// screenSplash is the optional first-paint shown briefly (≈600 ms) before
	// the age form. It exists so the very first thing the user sees is the
	// Sip-Happens wordmark, not a legal warning. Auto-advances to screenWelcome
	// via a splashAdvanceMsg tick — see Init().
	screenSplash
	screenError
)

// tokenScopePreset is the small set of pre-canned permission bundles offered
// in the TUI form. Power users can call POST /tokens directly with custom
// scopes (the API still validates against domain.PATScope*).
type tokenScopePreset int

const (
	tokenScopeReadOnly tokenScopePreset = iota
	tokenScopeFullAccess
)

func (p tokenScopePreset) scopes() []string {
	switch p {
	case tokenScopeFullAccess:
		return []string{
			"catalog:read",
			"profile:read", "profile:write",
			"cart:write",
			"orders:read", "orders:write",
			"subscriptions:read", "subscriptions:write",
		}
	default:
		return []string{
			"catalog:read",
			"profile:read",
			"orders:read",
			"subscriptions:read",
		}
	}
}

// tokenDraft captures the in-progress token creation form.
type tokenDraft struct {
	name   string
	preset tokenScopePreset
}

type paymentKind int

const (
	paymentKindOrder paymentKind = iota
	paymentKindSubscription
)

type subscriptionDraft struct {
	productID   string
	productName string
	currency    string
	unitAmount  int64
	qty         int
	interval    string // week | month
	addressID   string
}

const (
	catalogPageSize = 15
	// catalogFeaturedSize is the small "destacados" page surfaced on the
	// catalog tab toggle and the /view/init bootstrap.
	catalogFeaturedSize = 6
)

// catalogGroup selects which catalog slice the TUI is currently showing.
// Persisted on AppState so n/p paging stays within the active group.
type catalogGroup int

const (
	catalogGroupFeatured catalogGroup = iota
	catalogGroupAll
)

type CartLine struct {
	ProductID    string
	SKU          string
	Name         string
	Currency     string
	UnitPrice    int64
	Qty          int
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
	return L.CheckoutFieldLabels
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

	catalog           []api.CatalogItem
	catalogCursor     int
	catalogQuery      api.CatalogQuery
	catalogHasMore    bool
	catalogSearchEdit bool
	catalogSearchBuf  string
	catalogFilter     catalogFilterFields
	catalogGroup      catalogGroup
	productSKU        string
	productDetail     *api.CatalogDetail
	// productDetailCache memoises GET /catalog/{id} per session so
	// ↑/↓ browsing inside a single TUI run doesn't re-hit the network
	// for products the user already looked at. Keyed by the public id
	// the catalog list returned. Capped via len() < detailCacheMax to
	// keep long browsing sessions from growing the map unbounded.
	productDetailCache map[string]*api.CatalogDetail
	// profileIncomplete is true when the customer record returned by
	// /view/init has no name / email — the TUI shows a one-line banner
	// nudging the user to complete the account.
	profileIncomplete bool

	lastOrderID         string
	lastOrderDisplayRef string
	stripeURL           string
	orderStatus         string
	orderExpiresAt      *time.Time
	lastOrderTotal      int64
	lastOrderCurrency   string
	lastShipment        *api.ShipmentView
	clock               time.Time

	orderHistory       []api.OrderView
	orderHistoryCursor int
	// allowedOrderIDs tracks orders created this session or selected from history.
	allowedOrderIDs map[string]struct{}

	idempotencyKey string

	// Shipping estimate for checkout / cart (from GET /shipping/estimate).
	shippingMinor       int64
	shippingCurrency    string
	shippingEstimateErr string

	accountSection int

	savedAddresses    []api.AddressView
	addressCursor     int
	checkoutPickAddress bool

	subscriptions          []api.SubscriptionView
	subscriptionCursor     int
	subDraft               subscriptionDraft
	lastSubscriptionID     string
	lastSubscriptionStatus string
	paymentKind            paymentKind

	tokens         []api.PersonalAccessTokenView
	tokenCursor    int
	tokenDraft     tokenDraft
	// lastTokenRaw is the freshly minted PAT secret. Cleared as soon as the
	// user leaves screenTokenShow so it never lingers in memory.
	lastTokenRaw   string
	lastTokenView  *api.PersonalAccessTokenView
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

func (s *AppState) cartQtyForProduct(productID string) int {
	for _, l := range s.cart {
		if l.ProductID == productID {
			return l.Qty
		}
	}
	return 0
}

func (s *AppState) adjustCartQty(productID string, delta int, avail int) {
	if delta == 0 {
		return
	}
	for i := range s.cart {
		if s.cart[i].ProductID != productID {
			continue
		}
		s.cart[i].Qty += delta
		s.cart[i].AvailableQty = avail
		if s.cart[i].Qty <= 0 {
			s.cart = append(s.cart[:i], s.cart[i+1:]...)
		}
		return
	}
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
		return fmt.Errorf("%s", L.ErrCartEmpty)
	}
	for _, l := range s.cart {
		if l.Qty <= 0 {
			return fmt.Errorf("%s", L.ErrCartInvalidQty(l.SKU))
		}
		if l.AvailableQty >= 0 && l.Qty > l.AvailableQty {
			return fmt.Errorf("%s", L.ErrCartStock(l.SKU, l.AvailableQty, l.Qty))
		}
	}
	return nil
}

func (s *AppState) orderLabel() string {
	return formatOrderLabel(s.lastOrderID, s.lastOrderDisplayRef)
}

func (s *AppState) setOrderSnapshot(order api.OrderView, shipment *api.ShipmentView) {
	s.lastOrderDisplayRef = strings.TrimSpace(order.DisplayRef)
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
	return L.FilterFieldLabels
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
	s.catalogQuery = catalogQueryForGroup(s.catalogGroup)
	s.catalogHasMore = false
	s.catalogSearchBuf = ""
	s.catalogSearchEdit = false
}

// catalogQueryForGroup builds the default CatalogQuery for the given group
// (destacados vs full catalog). Used at first paint, after age-gate, and
// whenever the user toggles `g` on the catalog screen.
func catalogQueryForGroup(g catalogGroup) api.CatalogQuery {
	switch g {
	case catalogGroupFeatured:
		return api.CatalogQuery{Limit: catalogFeaturedSize, Offset: 0, Featured: true}
	default:
		return api.CatalogQuery{Limit: catalogPageSize, Offset: 0}
	}
}

const detailCacheMax = 50

// rememberProductDetail stores `d` in productDetailCache, evicting an
// arbitrary entry if the map has grown past detailCacheMax. Iteration over
// a Go map is intentionally non-deterministic — good enough for a
// per-session bounded cache.
func (s *AppState) rememberProductDetail(d *api.CatalogDetail) {
	if d == nil {
		return
	}
	key := strings.TrimSpace(d.Product.ID)
	if key == "" {
		key = strings.TrimSpace(d.Product.SKU)
	}
	if key == "" {
		return
	}
	if s.productDetailCache == nil {
		s.productDetailCache = make(map[string]*api.CatalogDetail, 16)
	}
	if len(s.productDetailCache) >= detailCacheMax {
		for k := range s.productDetailCache {
			delete(s.productDetailCache, k)
			break
		}
	}
	s.productDetailCache[key] = d
}

// productDetailFromCache returns the cached detail for `ref` if present.
func (s *AppState) productDetailFromCache(ref string) *api.CatalogDetail {
	if s.productDetailCache == nil {
		return nil
	}
	return s.productDetailCache[strings.TrimSpace(ref)]
}

