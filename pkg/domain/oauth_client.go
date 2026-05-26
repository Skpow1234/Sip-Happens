package domain

import "time"

// OAuthClientID identifies a row in oauth_clients (internal UUID).
type OAuthClientID string

// OAuthClient is a registered third-party application that may run the
// Authorization Code flow against /oauth/authorize + /oauth/token.
//
// The raw client secret is never persisted — only ClientSecretHash
// (SHA-256 of the issued secret string) and the short lookup
// ClientSecretPrefix (indexed for cheap auth lookups).
type OAuthClient struct {
	ID                 OAuthClientID
	CustomerID         CustomerID
	Name               string
	ClientID           string
	ClientSecretPrefix string
	ClientSecretHash   []byte
	RedirectURIs       []string
	// Scopes is the set of default capabilities the client may request at
	// authorize time; values are the same vocabulary as PersonalAccessTokenScope.
	Scopes       []PersonalAccessTokenScope
	Confidential bool
	RevokedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsActive reports whether the client may still be used at t.
func (c OAuthClient) IsActive(t time.Time) bool {
	return c.RevokedAt == nil
}
