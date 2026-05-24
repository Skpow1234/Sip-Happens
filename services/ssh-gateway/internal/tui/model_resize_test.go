package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestModel_WindowSize_AllScreens_ViewDoesNotPanic(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 120, Height: 40},
		{Width: 20, Height: 5},
		{Width: 1, Height: 1},
		{Width: 0, Height: 0},
		{Width: 200, Height: 60},
	}
	screens := []screen{
		screenWelcome, screenCatalog, screenCart,
		screenCheckout, screenConfirm, screenPayment, screenOrderStatus,
		screenAccount, screenSubscriptionConfirm, screenSubscriptionStatus,
		screenCatalogFilters, screenHelp, screenOrderComplete, screenError,
	}
	for _, sc := range screens {
		for _, sz := range sizes {
			m := seedModelForScreen(sc)
			next, _ := m.Update(sz)
			_ = next.(Model).View()
		}
	}
}

func TestModel_Init_ReturnsCommand(t *testing.T) {
	m := NewRootModel(nil, "", "", "", "", "")
	if cmd := m.Init(); cmd == nil {
		t.Fatal("expected init command")
	}
}

func seedModelForScreen(sc screen) Model {
	m := NewRootModel(nil, "", "", "", "", "")
	m.screen = sc
	m.width, m.height = 80, 24
	m.state.birthDateInput = "1995-01-01"
	m.state.catalog = []api.CatalogItem{makeCatalogItem("SKU-1")}
	m.state.productDetail = &api.CatalogDetail{}
	m.state.productDetail.Product.SKU = "SKU-1"
	m.state.productDetail.Product.Name = "Cerveza demo"
	m.state.cart = []CartLine{{SKU: "SKU-1", Qty: 1, UnitPrice: 1000, Currency: "COP"}}
	m.state.stripeURL = "https://checkout.stripe.test/session"
	m.state.orderStatus = "PENDING_PAYMENT"
	m.state.lastOrderID = "order-1"
	m.state.lastOrderDisplayRef = "SH-TEST"
	m.state.lastSubscriptionID = "sub_test"
	m.state.lastSubscriptionStatus = "PENDING"
	m.err = errForScreen(sc)
	return m
}

func errForScreen(sc screen) error {
	if sc == screenError {
		return errTestResize
	}
	return nil
}

var errTestResize = &testResizeErr{}

type testResizeErr struct{}

func (testResizeErr) Error() string { return "error de prueba" }
