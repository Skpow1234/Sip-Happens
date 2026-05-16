package shipping

import (
	"fmt"
	"strings"
)

// CanonicalCity returns the display name for checkout if the city is allowed (Bogotá, Medellín, Cali).
func CanonicalCity(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.ReplaceAll(s, "á", "a")
	s = strings.ReplaceAll(s, "í", "i")
	s = strings.ReplaceAll(s, "ó", "o")
	s = strings.ReplaceAll(s, "ú", "u")
	s = strings.ReplaceAll(s, "ñ", "n")
	switch s {
	case "bogota":
		return "Bogotá", nil
	case "medellin":
		return "Medellín", nil
	case "cali":
		return "Cali", nil
	default:
		return "", fmt.Errorf("delivery city must be Bogotá, Medellín, or Cali")
	}
}

// CityKey is the normalized key used in shipping_rates.city_normalized.
func CityKey(display string) string {
	c, err := CanonicalCity(display)
	if err != nil {
		return ""
	}
	switch c {
	case "Bogotá":
		return "bogota"
	case "Medellín":
		return "medellin"
	case "Cali":
		return "cali"
	default:
		return ""
	}
}
