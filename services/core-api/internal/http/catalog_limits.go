package http

import (
	"fmt"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
)

const (
	catalogDefaultLimit = 50
	catalogMaxLimit     = 100
)

func parseCatalogLimitQuery(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return catalogDefaultLimit, nil
	}
	n := parseIntDefault(raw, catalogDefaultLimit)
	if n < 1 {
		return 0, domain.ErrInvalidInput
	}
	if n > catalogMaxLimit {
		return 0, fmt.Errorf("limit must be <= %d", catalogMaxLimit)
	}
	return n, nil
}

// parseBoolQuery reads a "1 / true / yes / on" (case-insensitive) query flag
// and returns false for anything else, including empty / malformed values.
// Used for non-critical filters like ?featured=true where invalid input
// should silently fall back to the default behavior, not 422.
func parseBoolQuery(r *http.Request, name string) bool {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name)))
	switch raw {
	case "1", "t", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
