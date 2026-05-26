package tokens

import (
	"time"

	"sip-happens/pkg/domain"
)

// TokenView is the public JSON shape for a PAT.
//
// Note: token_hash is NEVER exposed and the raw secret only appears once, in the
// dedicated `token` field of the create response — see CreateResponseView.
type TokenView struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TokenViewFromDomain converts a domain token to its public JSON shape.
func TokenViewFromDomain(t domain.PersonalAccessToken) TokenView {
	scopes := make([]string, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, string(s))
	}
	return TokenView{
		ID:          PublicID(t.ID),
		Name:        t.Name,
		TokenPrefix: t.TokenPrefix,
		Scopes:      scopes,
		LastUsedAt:  t.LastUsedAt,
		ExpiresAt:   t.ExpiresAt,
		RevokedAt:   t.RevokedAt,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
