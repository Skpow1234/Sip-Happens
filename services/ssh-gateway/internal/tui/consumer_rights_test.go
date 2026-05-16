package tui

import (
	"strings"
	"testing"
)

func TestConsumerRightsURL_WelcomeCheckoutConfirm(t *testing.T) {
	url := "https://sic.example/consumer"
	m := NewRootModel(nil, "", url, "")
	m.state.birthDateInput = "1990-01-01"

	m.screen = screenWelcome
	if !strings.Contains(m.View(), url) || !strings.Contains(m.View(), "SIC") {
		t.Fatalf("welcome missing consumer rights: %q", m.View())
	}

	m.screen = screenMenu
	if !strings.Contains(m.View(), url) {
		t.Fatalf("menu missing consumer rights")
	}

	m.screen = screenCheckout
	m.state.checkout.city = "Bogotá"
	if !strings.Contains(m.View(), url) {
		t.Fatalf("checkout missing consumer rights")
	}

	m.screen = screenConfirm
	m.state.privacyAccepted = true
	if !strings.Contains(m.View(), url) {
		t.Fatalf("confirm missing consumer rights")
	}

	m.screen = screenOrderComplete
	m.state.lastOrderID = "ord-1"
	m.state.orderStatus = "PAID"
	if !strings.Contains(m.View(), url) {
		t.Fatalf("order complete missing consumer rights")
	}
}
