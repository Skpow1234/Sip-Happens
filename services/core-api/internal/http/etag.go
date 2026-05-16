package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sip-happens/pkg/domain"
)

func productWeakETag(updatedAt time.Time) string {
	return fmt.Sprintf(`W/"%d"`, updatedAt.UTC().UnixNano())
}

func parseProductIfMatch(header string, current time.Time) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return true
	}
	const weakPrefix = `W/"`
	if !strings.HasPrefix(header, weakPrefix) || !strings.HasSuffix(header, `"`) {
		return false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(header, weakPrefix), `"`)
	n, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return false
	}
	return n == current.UTC().UnixNano()
}

func (s *Server) checkAdminProductIfMatch(w http.ResponseWriter, r *http.Request, productID domain.ProductID) bool {
	if s.Admin == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return false
	}
	updatedAt, err := s.Admin.ProductUpdatedAt(r.Context(), productID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "product not found")
			return false
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return false
	}
	if !parseProductIfMatch(r.Header.Get("If-Match"), updatedAt) {
		writeAPIError(w, r, http.StatusPreconditionFailed, "precondition_failed", "If-Match does not match current product version")
		return false
	}
	return true
}

func (s *Server) writeAdminProductNoContent(w http.ResponseWriter, r *http.Request, productID domain.ProductID) {
	if s.Admin != nil {
		if updatedAt, err := s.Admin.ProductUpdatedAt(r.Context(), productID); err == nil {
			w.Header().Set("ETag", productWeakETag(updatedAt))
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
