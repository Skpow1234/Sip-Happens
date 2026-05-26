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
	"sip-happens/services/core-api/internal/tokens"
)

const (
	authorizationCodeTTL  = 10 * time.Minute
	oauthAccessTokenTTL   = 30 * 24 * time.Hour // 30 days
	oauthSupportedGrant   = "authorization_code"
	oauthTokenTypeBearer  = "Bearer"
)

// oauthError is a small typed wrapper so handlers can short-circuit with a
// known OAuth `error` code (RFC 6749 §5.2) without scattering string literals.
type oauthError struct {
	httpStatus  int
	code        string
	description string
}

func (e *oauthError) Error() string { return e.code + ": " + e.description }

var (
	errOAuthInvalidRequest      = &oauthError{httpStatus: http.StatusBadRequest, code: "invalid_request"}
	errOAuthInvalidClient       = &oauthError{httpStatus: http.StatusUnauthorized, code: "invalid_client"}
	errOAuthInvalidGrant        = &oauthError{httpStatus: http.StatusBadRequest, code: "invalid_grant"}
	errOAuthUnsupportedGrant    = &oauthError{httpStatus: http.StatusBadRequest, code: "unsupported_grant_type"}
	errOAuthInvalidScope        = &oauthError{httpStatus: http.StatusBadRequest, code: "invalid_scope"}
	errOAuthServerError         = &oauthError{httpStatus: http.StatusInternalServerError, code: "server_error"}
)

func writeOAuthError(w http.ResponseWriter, base *oauthError, description string) {
	status := base.httpStatus
	body := map[string]string{
		"error":             base.code,
		"error_description": description,
	}
	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.Header().Set("pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// --- /oauth/authorize (signed) ---

type authorizeRequest struct {
	ClientID     string   `json:"client_id"`
	RedirectURI  string   `json:"redirect_uri"`
	Scopes       []string `json:"scopes"`
	State        string   `json:"state"`
	ResponseType string   `json:"response_type"` // accepted but only "code" supported
}

func (s *Server) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	customerID, ok := s.customerIDFromRequest(w, r)
	if !ok {
		return
	}
	if s.OAuthClients == nil || s.OAuthCodes == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}

	var req authorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if rt := strings.TrimSpace(req.ResponseType); rt != "" && rt != "code" {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "response_type must be 'code'")
		return
	}

	client, err := s.OAuthClients.GetByClientID(r.Context(), s.DB, strings.TrimSpace(req.ClientID))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "oauth client not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if !oauthapps.AllowsRedirect(client, req.RedirectURI) {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "redirect_uri not registered for this client")
		return
	}

	scopes, err := normalizeOAuthScopes(req.Scopes)
	if err != nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", err.Error())
		return
	}
	if !oauthapps.ScopesContain(client, scopes) {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "scope set exceeds client's registered scopes")
		return
	}

	gen, err := oauthapps.GenerateAuthorizationCode()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	codeUUID, err := ids.UUIDv4()
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	now := time.Now()
	row := domain.OAuthAuthorizationCode{
		ID:            domain.OAuthAuthorizationCodeID(codeUUID),
		OAuthClientID: client.ID,
		CustomerID:    customerID,
		CodePrefix:    gen.Prefix,
		CodeHash:      gen.Hash,
		RedirectURI:   strings.TrimSpace(req.RedirectURI),
		Scopes:        scopes,
		ExpiresAt:     now.Add(authorizationCodeTTL),
		CreatedAt:     now,
	}
	if err := s.OAuthCodes.Create(r.Context(), s.DB, row); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(http.StatusCreated)
	out := map[string]any{
		"code":         gen.Raw,
		"expires_at":   row.ExpiresAt,
		"redirect_uri": row.RedirectURI,
		"client_id":    client.ClientID,
		"scopes":       scopeStrings(scopes),
	}
	if st := strings.TrimSpace(req.State); st != "" {
		out["state"] = st
	}
	_ = json.NewEncoder(w).Encode(out)
}

// --- /oauth/token (public, RFC 6749 form-encoded) ---

func (s *Server) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOAuthError(w, errOAuthInvalidRequest, "method must be POST")
		return
	}
	if s.OAuthClients == nil || s.OAuthCodes == nil || s.Tokens == nil || s.DB == nil {
		writeOAuthError(w, errOAuthServerError, "server misconfigured")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, errOAuthInvalidRequest, "could not parse form")
		return
	}
	grant := strings.TrimSpace(r.PostFormValue("grant_type"))
	if grant == "" {
		writeOAuthError(w, errOAuthInvalidRequest, "missing grant_type")
		return
	}
	if grant != oauthSupportedGrant {
		writeOAuthError(w, errOAuthUnsupportedGrant, "only authorization_code is supported")
		return
	}

	clientID, clientSecret, ok := extractClientCredentials(r)
	if !ok {
		writeOAuthError(w, errOAuthInvalidClient, "missing client credentials")
		return
	}
	rawCode := strings.TrimSpace(r.PostFormValue("code"))
	redirectURI := strings.TrimSpace(r.PostFormValue("redirect_uri"))
	if rawCode == "" || redirectURI == "" {
		writeOAuthError(w, errOAuthInvalidRequest, "missing code or redirect_uri")
		return
	}

	client, err := s.OAuthClients.GetByClientID(r.Context(), s.DB, clientID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeOAuthError(w, errOAuthInvalidClient, "unknown client")
			return
		}
		writeOAuthError(w, errOAuthServerError, "internal error")
		return
	}
	if !oauthapps.VerifyHash(clientSecret, client.ClientSecretHash) {
		writeOAuthError(w, errOAuthInvalidClient, "bad client secret")
		return
	}

	codePrefix, err := oauthapps.ParseAuthorizationCode(rawCode)
	if err != nil {
		writeOAuthError(w, errOAuthInvalidGrant, "malformed authorization code")
		return
	}
	codeRow, err := s.OAuthCodes.Consume(r.Context(), s.DB, codePrefix)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeOAuthError(w, errOAuthInvalidGrant, "code expired, already used, or unknown")
			return
		}
		writeOAuthError(w, errOAuthServerError, "internal error")
		return
	}
	if !oauthapps.VerifyHash(rawCode, codeRow.CodeHash) {
		writeOAuthError(w, errOAuthInvalidGrant, "code does not match prefix")
		return
	}
	if codeRow.OAuthClientID != client.ID {
		writeOAuthError(w, errOAuthInvalidGrant, "code was issued to a different client")
		return
	}
	if codeRow.RedirectURI != redirectURI {
		writeOAuthError(w, errOAuthInvalidGrant, "redirect_uri does not match the original authorize request")
		return
	}

	// Mint the access token, persisted in personal_access_tokens so the
	// (future) Bearer auth middleware has one lookup path for SSH-minted
	// PATs and OAuth-issued tokens alike. The token's Name records the
	// issuing OAuth client for human-readable audit until the upcoming
	// oauth-app-crud task adds a proper FK column.
	gen, err := tokens.GenerateRawToken()
	if err != nil {
		writeOAuthError(w, errOAuthServerError, "internal error")
		return
	}
	tokenUUID, err := ids.UUIDv4()
	if err != nil {
		writeOAuthError(w, errOAuthServerError, "internal error")
		return
	}
	now := time.Now()
	expires := now.Add(oauthAccessTokenTTL)
	pat := domain.PersonalAccessToken{
		ID:          domain.PersonalAccessTokenID(tokenUUID),
		CustomerID:  codeRow.CustomerID,
		Name:        "oauth:" + client.Name,
		TokenPrefix: gen.Prefix,
		TokenHash:   gen.Hash,
		Scopes:      codeRow.Scopes,
		ExpiresAt:   &expires,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.Tokens.Create(r.Context(), s.DB, pat); err != nil {
		writeOAuthError(w, errOAuthServerError, "could not persist access token")
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	w.Header().Set("pragma", "no-cache")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": gen.Raw,
		"token_type":   oauthTokenTypeBearer,
		"expires_in":   int(oauthAccessTokenTTL.Seconds()),
		"scope":        strings.Join(scopeStrings(codeRow.Scopes), " "),
	})
}

// extractClientCredentials honors both RFC 6749 §2.3 mechanisms:
//   - HTTP Basic auth (client_id:client_secret base64-encoded)
//   - form body fields `client_id` / `client_secret`
func extractClientCredentials(r *http.Request) (clientID, clientSecret string, ok bool) {
	if u, p, present := r.BasicAuth(); present {
		// RFC 6749 §2.3.1: form-encoded inside Basic auth.
		if decoded, err := url.QueryUnescape(u); err == nil {
			u = decoded
		}
		if decoded, err := url.QueryUnescape(p); err == nil {
			p = decoded
		}
		u = strings.TrimSpace(u)
		p = strings.TrimSpace(p)
		if u != "" && p != "" {
			return u, p, true
		}
	}
	id := strings.TrimSpace(r.PostFormValue("client_id"))
	secret := strings.TrimSpace(r.PostFormValue("client_secret"))
	if id == "" || secret == "" {
		return "", "", false
	}
	return id, secret, true
}

func normalizeOAuthScopes(raw []string) ([]domain.PersonalAccessTokenScope, error) {
	seen := make(map[domain.PersonalAccessTokenScope]struct{}, len(raw))
	out := make([]domain.PersonalAccessTokenScope, 0, len(raw))
	for _, r := range raw {
		s := domain.PersonalAccessTokenScope(strings.TrimSpace(r))
		if s == "" {
			continue
		}
		if _, ok := allowedTokenScopes[s]; !ok {
			return nil, errOAuthScope("unsupported scope: " + string(s))
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, errOAuthScope("scopes must include at least one capability")
	}
	return out, nil
}

func errOAuthScope(msg string) error { return &scopeError{msg: msg} }

func scopeStrings(scopes []domain.PersonalAccessTokenScope) []string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, string(s))
	}
	return out
}
