package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/auth"
	"sip-happens/pkg/domain"
	"sip-happens/pkg/obslog"
	"sip-happens/services/core-api/internal/tokens"
)

type patAuthContextKey int

const (
	patScopesKey patAuthContextKey = iota + 1
	patCustomerIDKey
	patAuthenticatedKey
)

// withPATAuth attaches customer id + scopes for a successful Bearer-PAT auth.
func withPATAuth(ctx context.Context, customerID domain.CustomerID, scopes []domain.PersonalAccessTokenScope) context.Context {
	ctx = context.WithValue(ctx, patAuthenticatedKey, true)
	ctx = context.WithValue(ctx, patCustomerIDKey, customerID)
	ctx = context.WithValue(ctx, patScopesKey, scopes)
	return ctx
}

// patScopesFromContext returns scopes when the request was authenticated via PAT.
func patScopesFromContext(ctx context.Context) ([]domain.PersonalAccessTokenScope, bool) {
	if ctx.Value(patAuthenticatedKey) != true {
		return nil, false
	}
	scopes, _ := ctx.Value(patScopesKey).([]domain.PersonalAccessTokenScope)
	return scopes, true
}

func patCustomerIDFromContext(ctx context.Context) (domain.CustomerID, bool) {
	if ctx.Value(patAuthenticatedKey) != true {
		return "", false
	}
	id, ok := ctx.Value(patCustomerIDKey).(domain.CustomerID)
	return id, ok && id != ""
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

// scopeAllows reports whether granted scopes satisfy need.
// Write scopes imply their matching read scope (orders:write → orders:read, etc.).
func scopeAllows(granted []domain.PersonalAccessTokenScope, need domain.PersonalAccessTokenScope) bool {
	implied := map[domain.PersonalAccessTokenScope]domain.PersonalAccessTokenScope{
		domain.PATScopeOrdersWrite:        domain.PATScopeOrdersRead,
		domain.PATScopeSubscriptionsWrite: domain.PATScopeSubscriptionsRead,
		domain.PATScopeProfileWrite:       domain.PATScopeProfileRead,
	}
	for _, g := range granted {
		if g == need {
			return true
		}
		if read, ok := implied[g]; ok && read == need {
			return true
		}
	}
	return false
}

// requirePATScope enforces need when the caller authenticated with a PAT.
// HMAC / unsigned (dev) callers are unrestricted.
func (s *Server) requirePATScope(need domain.PersonalAccessTokenScope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scopes, isPAT := patScopesFromContext(r.Context())
		if isPAT && !scopeAllows(scopes, need) {
			writeAPIError(w, r, http.StatusForbidden, "forbidden", "insufficient_scope")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requirePATScopeRW picks read vs write scope from the HTTP method.
func (s *Server) requirePATScopeRW(read, write domain.PersonalAccessTokenScope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scopes, isPAT := patScopesFromContext(r.Context())
		if isPAT {
			need := write
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				need = read
			}
			if !scopeAllows(scopes, need) {
				writeAPIError(w, r, http.StatusForbidden, "forbidden", "insufficient_scope")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// serveWithPAT authenticates Authorization: Bearer pat_… and continues.
func (s *Server) serveWithPAT(w http.ResponseWriter, r *http.Request, raw string, next http.Handler) {
	if s.Tokens == nil || s.DB == nil || s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	prefix, err := tokens.ParseRawToken(raw)
	if err != nil {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "invalid_token")
		return
	}
	tok, err := s.Tokens.GetByPrefix(r.Context(), s.DB, prefix)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "invalid_token")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if !tokens.VerifyRawToken(raw, tok.TokenHash) {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "invalid_token")
		return
	}
	if !tok.IsActive(time.Now()) {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "invalid_token")
		return
	}

	cust, err := s.Customers.GetByID(r.Context(), tok.CustomerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "invalid_token")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if cust == nil {
		writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "invalid_token")
		return
	}

	// Optional fingerprint tag must match the token owner when provided.
	if fp := strings.TrimSpace(r.Header.Get(auth.HeaderFingerprint)); fp != "" {
		if fp != strings.TrimSpace(cust.SSHFingerprint) {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "fingerprint_mismatch")
			return
		}
	}

	// Existing handlers resolve the customer via X-SSH-Fingerprint. Inject the
	// owner's fingerprint so PAT and HMAC paths share the same identity plumbing.
	r2 := r.Clone(r.Context())
	r2.Header = r.Header.Clone()
	if fp := strings.TrimSpace(cust.SSHFingerprint); fp != "" {
		r2.Header.Set(auth.HeaderFingerprint, fp)
	}

	ctx := withPATAuth(r2.Context(), tok.CustomerID, tok.Scopes)
	ctx = obslog.WithCustomerID(ctx, string(tok.CustomerID))
	r2 = r2.WithContext(ctx)

	_ = s.Tokens.TouchLastUsed(r.Context(), s.DB, prefix)

	next.ServeHTTP(w, r2)
}
