package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"sip-happens/services/ssh-gateway/internal/api"
)

// TestDetailCacheSkipsRefetch checks that once a product detail has been
// recorded via rememberProductDetail, maybeLoadCatalogDetailCmd does NOT
// emit a fetch command for the same row — the cache is the whole point
// of the tui-detail-cache polish step.
func TestDetailCacheSkipsRefetch(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.state.catalog = []api.CatalogItem{
		{Product: api.CatalogProduct{ID: "prd_x", SKU: "SKU-X", Name: "X"}},
	}
	m.state.catalogCursor = 0
	m.state.rememberProductDetail(&api.CatalogDetail{
		Product: api.CatalogProduct{ID: "prd_x", SKU: "SKU-X", Name: "X"},
	})
	if cmd := m.maybeLoadCatalogDetailCmd(); cmd != nil {
		t.Fatalf("expected no fetch command on cache hit, got %T", cmd)
	}
	if m.state.productDetail == nil {
		t.Fatal("expected productDetail hydrated from cache")
	}
	if m.busy {
		t.Fatal("cache hit must not flip busy=true")
	}
}

// TestDetailCacheEvictsWhenFull guards against unbounded growth.
func TestDetailCacheEvictsWhenFull(t *testing.T) {
	s := AppState{}
	for i := 0; i < detailCacheMax+10; i++ {
		s.rememberProductDetail(&api.CatalogDetail{
			Product: api.CatalogProduct{ID: "prd_" + string(rune('a'+i%26)) + intStr(i)},
		})
	}
	if len(s.productDetailCache) > detailCacheMax {
		t.Fatalf("cache grew past max: got %d", len(s.productDetailCache))
	}
}

func intStr(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// TestPaymentSessionExpiredFlag verifies the PENDING_PAYMENT past-expiry
// detection: the trigger for the new "sesión vencida" banner + `c` reset.
func TestPaymentSessionExpiredFlag(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	future := time.Now().Add(10 * time.Minute)

	cases := []struct {
		name   string
		status string
		exp    *time.Time
		want   bool
	}{
		{"past PENDING_PAYMENT", "PENDING_PAYMENT", &past, true},
		{"future PENDING_PAYMENT", "PENDING_PAYMENT", &future, false},
		{"PAID past expiry", "PAID", &past, false},
		{"no expiry", "PENDING_PAYMENT", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewRootModel(nil, "", "", "", "", "")
			m.state.orderStatus = c.status
			m.state.orderExpiresAt = c.exp
			if got := m.paymentSessionExpired(); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestPaymentExpiredViewHidesQR confirms that once the session has lapsed
// the rendered payment screen no longer shows the (now-broken) Stripe URL
// — it shows the explicit "vencida" banner + recovery hint instead.
func TestPaymentExpiredViewHidesQR(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenPayment
	m.width, m.height = 100, 30
	m.state.lastOrderID = "ord_x"
	m.state.lastOrderDisplayRef = "SH-EXP"
	m.state.orderStatus = "PENDING_PAYMENT"
	m.state.orderExpiresAt = &past
	m.state.stripeURL = "https://checkout.stripe.test/expired"

	view := m.viewPaymentV2()
	if !strings.Contains(view, "vencida") && !strings.Contains(view, "venció") {
		t.Fatalf("expected expired banner in view, got: %q", view)
	}
	if strings.Contains(view, "checkout.stripe.test") {
		t.Fatalf("expired view leaked the (broken) Stripe URL: %q", view)
	}
}

// TestErrMsgOnPaymentScreenIsSilent ensures a transient errMsg arriving
// while on screenPayment does NOT clobber the QR with an error popup —
// the polling loop should silently re-arm so the user keeps seeing the QR.
func TestErrMsgOnPaymentScreenIsSilent(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenPayment
	m.state.lastOrderID = "ord_x"
	m.state.orderStatus = "PENDING_PAYMENT"

	next, cmd := m.Update(errMsg{err: errResize{}})
	nm := next.(Model)
	if nm.notice != "" {
		t.Fatalf("payment-screen errMsg surfaced as notice: %q", nm.notice)
	}
	if cmd == nil {
		t.Fatal("expected a re-armed poll command, got nil")
	}
}

type errResize struct{}

func (errResize) Error() string { return "transient" }

// TestProfileLooksIncompleteFromViewInit drives the ssh-key banner trigger.
func TestProfileLooksIncompleteFromViewInit(t *testing.T) {
	if profileLooksIncomplete(nil) {
		t.Fatal("nil profile must not trigger banner")
	}
	if !profileLooksIncomplete(&api.CustomerView{}) {
		t.Fatal("empty profile should trigger banner")
	}
	if profileLooksIncomplete(&api.CustomerView{FullName: "Ana"}) {
		t.Fatal("profile with name must not trigger banner")
	}
	if profileLooksIncomplete(&api.CustomerView{Email: "a@b"}) {
		t.Fatal("profile with email must not trigger banner")
	}
	if profileLooksIncomplete(&api.CustomerView{LegalID: "CC-123"}) {
		t.Fatal("profile with legal ID must not trigger banner")
	}
}

// TestCatalogGroupToggleResetsPaging confirms `g` flips destacados <-> all
// and resets the offset so n/p continue inside the new group.
func TestCatalogGroupToggleResetsPaging(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.state.catalogGroup = catalogGroupFeatured
	m.state.catalogQuery = catalogQueryForGroup(catalogGroupFeatured)
	m.state.catalogQuery.Offset = 12

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	nm := next.(Model)
	if nm.state.catalogGroup != catalogGroupAll {
		t.Fatalf("expected group all after toggle, got %v", nm.state.catalogGroup)
	}
	if nm.state.catalogQuery.Offset != 0 {
		t.Fatalf("expected offset reset to 0, got %d", nm.state.catalogQuery.Offset)
	}
	if nm.state.catalogQuery.Featured {
		t.Fatal("expected Featured=false in catálogo group")
	}
	if cmd == nil {
		t.Fatal("expected loadCatalogCmd after toggle")
	}
}

// TestCatalogQueryForGroupSetsFeaturedFlag ensures the wire param is
// driven solely by the active group regardless of leftover state.
func TestCatalogQueryForGroupSetsFeaturedFlag(t *testing.T) {
	q := catalogQueryForGroup(catalogGroupFeatured)
	if !q.Featured {
		t.Fatal("destacados group should set Featured=true")
	}
	if q.Limit != catalogFeaturedSize {
		t.Fatalf("destacados limit: got %d, want %d", q.Limit, catalogFeaturedSize)
	}

	q = catalogQueryForGroup(catalogGroupAll)
	if q.Featured {
		t.Fatal("all group must NOT request featured")
	}
	if q.Limit != catalogPageSize {
		t.Fatalf("all limit: got %d, want %d", q.Limit, catalogPageSize)
	}
}

// TestShipmentLabelLocalizesNewStates is the test for tui-shipment-states.
// PACKED / HANDED_OVER / FAILED must render their Spanish equivalents
// instead of the raw upper-case status code.
func TestShipmentLabelLocalizesNewStates(t *testing.T) {
	cases := map[string]string{
		"NEW":         "pendiente",
		"PENDING":     "pendiente", // backward-compat alias
		"PACKED":      "empacado",
		"HANDED_OVER": "entregado al transportador",
		"FAILED":      "fallido",
		"IN_TRANSIT":  "en ruta",
		"DELIVERED":   "entregado",
	}
	for raw, want := range cases {
		got := L.ShipmentStatusLabel(raw)
		if got != want {
			t.Errorf("ShipmentStatusLabel(%q) = %q, want %q", raw, got, want)
		}
	}
	out := formatShipmentSummary("PAID", &api.ShipmentView{Status: "HANDED_OVER", TrackingCode: "ABC"})
	if !strings.Contains(out, "entregado al transportador") {
		t.Fatalf("expected localized HANDED_OVER label, got: %q", out)
	}
	if !strings.Contains(out, "ABC") {
		t.Fatalf("expected tracking code in summary, got: %q", out)
	}
	failedOut := formatShipmentSummary("PAID", &api.ShipmentView{Status: "FAILED"})
	if !strings.Contains(failedOut, "fallido") {
		t.Fatalf("expected fallido in FAILED summary, got: %q", failedOut)
	}
}

// TestViewInitHydratesAllSections walks the bootstrap message through Update
// and verifies that cart, catalog and profile-incomplete flag all settle
// into AppState in a single round-trip.
func TestViewInitHydratesAllSections(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.busy = true

	view := &api.ViewInit{
		Profile: &api.CustomerView{ID: "cus_x"}, // empty → incomplete
		Catalog: &api.CatalogPage{
			Items: []api.CatalogItem{
				{Product: api.CatalogProduct{ID: "prd_a", Name: "A"}},
			},
			HasMore: true,
		},
		Cart: &api.CartView{},
	}
	next, _ := m.Update(viewInitLoadedMsg{view: view})
	nm := next.(Model)
	if !nm.state.profileIncomplete {
		t.Fatal("expected profileIncomplete from empty profile")
	}
	if len(nm.state.catalog) != 1 || nm.state.catalog[0].Product.ID != "prd_a" {
		t.Fatalf("catalog not hydrated: %+v", nm.state.catalog)
	}
	if !nm.state.catalogHasMore {
		t.Fatal("catalogHasMore should be true")
	}
	// busy is intentionally re-flipped to true by maybeLoadCatalogDetailCmd
	// when there's a cache miss for the first product — the bootstrap
	// happy-path immediately prefetches the first detail so the right
	// pane isn't empty on first paint. We don't assert busy here.
}

// TestViewInitErrorFallsBackToSequential ensures a /view/init error does
// not strand the user on a blank catalog — Update must dispatch the legacy
// cart + catalog loaders so they can still shop.
func TestViewInitErrorFallsBackToSequential(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = screenCatalog
	m.busy = true

	next, cmd := m.Update(viewInitLoadedMsg{err: errResize{}})
	nm := next.(Model)
	if nm.busy {
		t.Fatal("busy must clear before falling back")
	}
	if cmd == nil {
		t.Fatal("expected fallback batch command, got nil")
	}
}

// TestSplashAdvancesToWelcome verifies the new screenSplash auto-advances.
func TestSplashAdvancesToWelcome(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	if m.screen != screenSplash {
		t.Fatalf("expected initial screenSplash, got %v", m.screen)
	}
	next, _ := m.Update(splashAdvanceMsg{})
	if next.(Model).screen != screenWelcome {
		t.Fatalf("expected screenWelcome after advance, got %v", next.(Model).screen)
	}
	// Subsequent advance after the user already moved past welcome must
	// be a no-op (don't yank them back).
	m2 := NewRootModel(nil, "", "", "", "", "")
	m2.screen = screenCatalog
	next2, _ := m2.Update(splashAdvanceMsg{})
	if next2.(Model).screen != screenCatalog {
		t.Fatal("splashAdvanceMsg must not affect non-splash screens")
	}
}
