package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/tokens"
)

// allowedTokenScopes mirrors pkg/domain.PATScope* and rejects anything else.
// Kept here so the HTTP layer can return a 422 before touching the DB.
var allowedTokenScopes = map[domain.PersonalAccessTokenScope]struct{}{
	domain.PATScopeCatalogRead:        {},
	domain.PATScopeCartWrite:          {},
	domain.PATScopeOrdersRead:         {},
	domain.PATScopeOrdersWrite:        {},
	domain.PATScopeSubscriptionsRead:  {},
	domain.PATScopeSubscriptionsWrite: {},
	domain.PATScopeProfileRead:        {},
	domain.PATScopeProfileWrite:       {},
}

const (
	tokenNameMaxLen     = 80
	tokenMaxLifetimeDur = 365 * 24 * time.Hour
)

func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleTokensList(w, r)
	case http.MethodPost:
		s.handleTokenCreate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTokenByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleTokenGet(w, r)
	case http.MethodDelete:
		s.handleTokenRevoke(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTokensList(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Tokens == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	limit := parseLimitQuery(r, 20, 100)
	offset := parseOffsetQuery(r)
	page, err := s.Tokens.ListByCustomer(r.Context(), s.DB, customerID, limit, offset)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	items := make([]tokens.TokenView, 0, len(page.Tokens))
	for _, t := range page.Tokens {
		items = append(items, tokens.TokenViewFromDomain(t))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"personal_access_tokens": items,
		"limit":                  limit,
		"offset":                 offset,
		"has_more":               page.HasMore,
	})
}

type createTokenRequest struct {
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func (s *Server) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.Tokens == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "name is required")
		return
	}
	if len(name) > tokenNameMaxLen {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "name too long")
		return
	}

	scopes, err := normalizeRequestedScopes(req.Scopes)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}

	now := time.Now()
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		exp := req.ExpiresAt.UTC()
		if !exp.After(now) {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "expires_at must be in the future")
			return
		}
		if exp.Sub(now) > tokenMaxLifetimeDur {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "expires_at must be within 1 year")
			return
		}
		expiresAt = &exp
	}

	gen, err := tokens.GenerateRawToken()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	tokenUUID, err := ids.UUIDv4()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	t := domain.PersonalAccessToken{
		ID:          domain.PersonalAccessTokenID(tokenUUID),
		CustomerID:  customerID,
		Name:        name,
		TokenPrefix: gen.Prefix,
		TokenHash:   gen.Hash,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.Tokens.Create(r.Context(), s.DB, t); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"personal_access_token": tokens.TokenViewFromDomain(t),
		"token":                 gen.Raw,
	})
}

func (s *Server) handleTokenGet(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/tokens/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing token id")
		return
	}
	if s.Tokens == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	t, err := tokens.ResolveToken(r.Context(), s.Tokens, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "token not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"personal_access_token": tokens.TokenViewFromDomain(*t),
	})
}

func (s *Server) handleTokenRevoke(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/tokens/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing token id")
		return
	}
	if s.Tokens == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	cur, err := tokens.ResolveToken(r.Context(), s.Tokens, s.DB, customerID, ref)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "token not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	updated, err := s.Tokens.Revoke(r.Context(), s.DB, customerID, cur.ID)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "token not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"personal_access_token": tokens.TokenViewFromDomain(*updated),
	})
}

// normalizeRequestedScopes trims, deduplicates, and validates requested scope strings.
// An empty/missing list is rejected so users always make an explicit grant.
func normalizeRequestedScopes(raw []string) ([]domain.PersonalAccessTokenScope, error) {
	seen := make(map[domain.PersonalAccessTokenScope]struct{}, len(raw))
	out := make([]domain.PersonalAccessTokenScope, 0, len(raw))
	for _, r := range raw {
		s := domain.PersonalAccessTokenScope(strings.TrimSpace(r))
		if s == "" {
			continue
		}
		if _, ok := allowedTokenScopes[s]; !ok {
			return nil, errInvalidScope(s)
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errEmptyScopes
	}
	return out, nil
}

// scopeError keeps token validation errors plain string-typed so callers can
// surface them directly in the API response without leaking internal types.
type scopeError struct{ msg string }

func (e *scopeError) Error() string { return e.msg }

var errEmptyScopes = &scopeError{msg: "scopes must include at least one capability"}

func errInvalidScope(s domain.PersonalAccessTokenScope) error {
	return &scopeError{msg: "unsupported scope: " + string(s)}
}
