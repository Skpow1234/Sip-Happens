package tui

import (
	"testing"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestAppState_orderIDAllowed(t *testing.T) {
	s := &AppState{}
	if s.orderIDAllowed("any") {
		t.Fatal("expected false with empty state")
	}
	s.rememberOrderID("order-1")
	if !s.orderIDAllowed("order-1") {
		t.Fatal("expected remembered order")
	}
	s.orderHistory = []api.OrderView{{ID: "order-2", Status: "PAID"}}
	if !s.orderIDAllowed("order-2") {
		t.Fatal("expected order from history list")
	}
	if s.orderIDAllowed("order-3") {
		t.Fatal("expected false for unknown order")
	}
}
