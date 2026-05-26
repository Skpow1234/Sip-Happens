package domain

import "time"

// OAuthAuthorizationCodeID identifies a row in oauth_authorization_codes.
type OAuthAuthorizationCodeID string

// OAuthAuthorizationCode is a short-lived, single-use credential minted by
// POST /oauth/authorize and consumed by POST /oauth/token to exchange for an
// access token. The raw `oac_<prefix>_<secret>` string never appears in the
// database — only the SHA-256 hash plus the indexed prefix.
type OAuthAuthorizationCode struct {
	ID            OAuthAuthorizationCodeID
	OAuthClientID OAuthClientID
	CustomerID    CustomerID
	CodePrefix    string
	CodeHash      []byte
	RedirectURI   string
	Scopes        []PersonalAccessTokenScope
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
	CreatedAt     time.Time
}

// IsRedeemable reports whether the code can still be exchanged at t.
func (c OAuthAuthorizationCode) IsRedeemable(t time.Time) bool {
	if c.ConsumedAt != nil {
		return false
	}
	return c.ExpiresAt.After(t)
}
