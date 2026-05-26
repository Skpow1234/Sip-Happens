package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/oauthapps"
)

const (
	oauthClientNameMaxLen     = 80
	oauthClientMaxRedirectURI = 512
	oauthClientMaxRedirectURIs = 8
)

func (s *Server) handleOAuthClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleOAuthClientsList(w, r)
	case http.MethodPost:
		s.handleOAuthClientCreate(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOAuthClientByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleOAuthClientGet(w, r)
	case http.MethodDelete:
		s.handleOAuthClientRevoke(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOAuthClientsList(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.OAuthClients == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	limit := parseLimitQuery(r, 20, 100)
	offset := parseOffsetQuery(r)
	page, err := s.OAuthClients.ListByCustomer(r.Context(), s.DB, customerID, limit, offset)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	items := make([]oauthapps.ClientView, 0, len(page.Clients))
	for _, c := range page.Clients {
		items = append(items, oauthapps.ViewFromDomain(c))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"oauth_clients": items,
		"limit":         limit,
		"offset":        offset,
		"has_more":      page.HasMore,
	})
}

type createOAuthClientRequest struct {
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
	Scopes       []string `json:"scopes"`
	Confidential *bool    `json:"confidential"`
}

func (s *Server) handleOAuthClientCreate(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.OAuthClients == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	var req createOAuthClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "name is required")
		return
	}
	if len(name) > oauthClientNameMaxLen {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "name too long")
		return
	}

	redirects, err := normalizeRedirectURIs(req.RedirectURIs)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}

	scopes, err := normalizeRequestedScopes(req.Scopes)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}

	confidential := true
	if req.Confidential != nil {
		confidential = *req.Confidential
	}

	clientIDStr, err := oauthapps.GenerateClientID()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	secret, err := oauthapps.GenerateClientSecret()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	rowUUID, err := ids.UUIDv4()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	now := time.Now()
	row := domain.OAuthClient{
		ID:                 domain.OAuthClientID(rowUUID),
		CustomerID:         customerID,
		Name:               name,
		ClientID:           clientIDStr,
		ClientSecretPrefix: secret.Prefix,
		ClientSecretHash:   secret.Hash,
		RedirectURIs:       redirects,
		Scopes:             scopes,
		Confidential:       confidential,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.OAuthClients.Create(r.Context(), s.DB, row); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(oauthapps.CreateResult{
		Client:       oauthapps.ViewFromDomain(row),
		ClientSecret: secret.Raw,
	})
}

func (s *Server) handleOAuthClientGet(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/oauth/clients/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing oauth client id")
		return
	}
	if s.OAuthClients == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	c, err := oauthapps.ResolveClient(r.Context(), s.OAuthClients, s.DB, customerID, ref)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "oauth client not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"oauth_client": oauthapps.ViewFromDomain(*c),
	})
}

func (s *Server) handleOAuthClientRevoke(w http.ResponseWriter, r *http.Request) {
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/oauth/clients/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing oauth client id")
		return
	}
	if s.OAuthClients == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	cur, err := oauthapps.ResolveClient(r.Context(), s.OAuthClients, s.DB, customerID, ref)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "oauth client not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	updated, err := s.OAuthClients.Revoke(r.Context(), s.DB, customerID, cur.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "oauth client not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"oauth_client": oauthapps.ViewFromDomain(*updated),
	})
}

// normalizeRedirectURIs enforces basic hygiene: at least one entry, no
// duplicates, https-only (or http://localhost for local dev tooling — the
// PKCE-less Authorization Code flow we run mandates HTTPS in production, but
// loopback http is permitted by RFC 8252 §7.3), reasonable length cap, and
// the count cap from the DB. Returns the deduped slice in insertion order.
func normalizeRedirectURIs(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, errRedirectURIs("at least one redirect_uri is required")
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		s := strings.TrimSpace(r)
		if s == "" {
			continue
		}
		if len(s) > oauthClientMaxRedirectURI {
			return nil, errRedirectURIs("redirect_uri too long")
		}
		u, err := url.Parse(s)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, errRedirectURIs("redirect_uri must be an absolute URL: " + s)
		}
		switch u.Scheme {
		case "https":
			// always fine
		case "http":
			host := u.Hostname()
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				return nil, errRedirectURIs("http:// is only permitted for loopback hosts: " + s)
			}
		default:
			return nil, errRedirectURIs("unsupported redirect_uri scheme: " + u.Scheme)
		}
		if u.Fragment != "" {
			// RFC 6749 §3.1.2: redirect URIs must not contain a fragment.
			return nil, errRedirectURIs("redirect_uri must not contain a fragment: " + s)
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errRedirectURIs("at least one redirect_uri is required")
	}
	if len(out) > oauthClientMaxRedirectURIs {
		return nil, errRedirectURIs("too many redirect_uris")
	}
	return out, nil
}

type redirectURIError struct{ msg string }

func (e *redirectURIError) Error() string { return e.msg }

func errRedirectURIs(msg string) error { return &redirectURIError{msg: msg} }
