package tui

import (
	"strings"
	"testing"

	"sip-happens/services/ssh-gateway/internal/api"
)

func TestFormatAddressLine_default(t *testing.T) {
	line := formatAddressLine(api.AddressView{
		Label:        "Casa",
		City:         "bogota",
		Neighborhood: "Chapinero",
		IsDefault:    true,
	})
	if !strings.HasPrefix(line, "● ") {
		t.Fatalf("expected default marker, got %q", line)
	}
	if !strings.Contains(line, "Casa") || !strings.Contains(line, "bogota") {
		t.Fatalf("unexpected line: %q", line)
	}
}

func TestApplyCheckoutAddress(t *testing.T) {
	var s AppState
	s.applyCheckoutAddress(api.AddressView{
		City:         "medellin",
		Neighborhood: "Poblado",
		AddressLine1: "Calle 10",
		Phone:        "300",
		LegalID:      "123",
		FullName:     "Ana",
	})
	if s.checkout.city != "medellin" || s.checkout.fullName != "Ana" {
		t.Fatalf("checkout not filled: %+v", s.checkout)
	}
}

func TestDefaultAddressIndex(t *testing.T) {
	idx := defaultAddressIndex([]api.AddressView{
		{IsDefault: false},
		{IsDefault: true},
	})
	if idx != 1 {
		t.Fatalf("got %d", idx)
	}
}
