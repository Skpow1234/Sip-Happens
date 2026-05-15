package tui

import (
	"fmt"
	"strings"
)

func formatMoney(currency string, minor int64) string {
	if currency == "" {
		currency = "COP"
	}
	// COP on Stripe is pesos (zero-decimal).
	s := fmt.Sprintf("%d", minor)
	if len(s) > 3 {
		var b strings.Builder
		for i, r := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteRune('.')
			}
			b.WriteRune(r)
		}
		s = b.String()
	}
	return fmt.Sprintf("%s %s", currency, s)
}
