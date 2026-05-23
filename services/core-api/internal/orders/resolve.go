package orders

import "strings"

// NormalizeOrderRef trims a path or query reference for order lookup.
func NormalizeOrderRef(ref string) string {
	return strings.TrimSpace(ref)
}

// RefLooksLikeDisplayRef reports whether ref is a human display reference (SH-…).
func RefLooksLikeDisplayRef(ref string) bool {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	return strings.HasPrefix(ref, "SH-") && len(ref) > 3
}
