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
