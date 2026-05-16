package tui

import (
	"strings"
	"testing"
)

func TestConsumerRightsURL_WelcomeCheckoutConfirm(t *testing.T) {
	url := "https://sic.example/consumer"
	m := NewRootModel(nil, "", url, "", "", "")
	m.state.birthDateInput = "1990-01-01"

	containsURL := func(view string) bool {
		return strings.Contains(stripANSI(view), url)
	}

	m.screen = screenWelcome
	welcome := m.View()
	if !containsURL(welcome) || !strings.Contains(stripANSI(welcome), "SIC") {
		t.Fatalf("welcome missing consumer rights: %q", welcome)
	}

	m.screen = screenAccount
	m.state.accountSection = accountAbout
	if !containsURL(m.View()) {
		t.Fatalf("account about missing consumer rights")
	}

	m.screen = screenCheckout
	m.state.checkout.city = "Bogotá"
	if !containsURL(m.View()) {
		t.Fatalf("checkout missing consumer rights")
	}

	m.screen = screenConfirm
	m.state.privacyAccepted = true
	if !containsURL(m.View()) {
		t.Fatalf("confirm missing consumer rights")
	}

	m.screen = screenOrderComplete
	m.state.lastOrderID = "ord-1"
	m.state.orderStatus = "PAID"
	if !containsURL(m.View()) {
		t.Fatalf("order complete missing consumer rights")
	}
}
