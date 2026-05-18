package tui

import (
	"strings"

	"sip-happens/services/ssh-gateway/internal/api"
)

func formatAddressLine(a api.AddressView) string {
	label := strings.TrimSpace(a.Label)
	if label == "" {
		label = strings.TrimSpace(a.AddressLine1)
	}
	if label == "" {
		label = "Dirección"
	}
	prefix := "  "
	if a.IsDefault {
		prefix = "● "
	}
	return prefix + label + " — " + a.City + ", " + a.Neighborhood
}

func (s *AppState) applyCheckoutAddress(a api.AddressView) {
	f := &s.checkout
	f.city = a.City
	f.neighborhood = a.Neighborhood
	f.address = a.AddressLine1
	f.phone = a.Phone
	f.legalID = a.LegalID
	f.fullName = a.FullName
}

func defaultAddressIndex(addrs []api.AddressView) int {
	for i, a := range addrs {
		if a.IsDefault {
			return i
		}
	}
	return 0
}
