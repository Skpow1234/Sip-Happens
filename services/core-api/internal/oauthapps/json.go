package oauthapps

import (
	"time"

	"sip-happens/pkg/domain"
)

// ClientView is the safe-to-emit JSON representation of an OAuth client.
//
// SECURITY: never include ClientSecretHash, ClientSecretPrefix raw bytes, or
// any value derived from the issued client_secret beyond confirming the
// caller's own row metadata. The raw secret is returned exactly once at
// creation time (see CreateResult) and then forever discarded by the server.
type ClientView struct {
	ID           string                            `json:"id"`
	ClientID     string                            `json:"client_id"`
	Name         string                            `json:"name"`
	RedirectURIs []string                          `json:"redirect_uris"`
	Scopes       []domain.PersonalAccessTokenScope `json:"scopes"`
	Confidential bool                              `json:"confidential"`
	RevokedAt    *time.Time                        `json:"revoked_at,omitempty"`
	CreatedAt    time.Time                         `json:"created_at"`
	UpdatedAt    time.Time                         `json:"updated_at"`
}

// CreateResult is the one-shot envelope returned by POST /oauth/clients —
// the raw client_secret is shown here exactly once. Subsequent reads (List,
// Get) never include it.
type CreateResult struct {
	Client       ClientView `json:"oauth_client"`
	ClientSecret string     `json:"client_secret"`
}

// ViewFromDomain projects a domain.OAuthClient into its public ClientView.
func ViewFromDomain(c domain.OAuthClient) ClientView {
	redirects := append([]string(nil), c.RedirectURIs...)
	scopes := append([]domain.PersonalAccessTokenScope(nil), c.Scopes...)
	if redirects == nil {
		redirects = []string{}
	}
	if scopes == nil {
		scopes = []domain.PersonalAccessTokenScope{}
	}
	return ClientView{
		ID:           PublicID(c.ID),
		ClientID:     c.ClientID,
		Name:         c.Name,
		RedirectURIs: redirects,
		Scopes:       scopes,
		Confidential: c.Confidential,
		RevokedAt:    c.RevokedAt,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}
