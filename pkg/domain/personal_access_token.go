package domain

import "time"

// PersonalAccessTokenID identifies a row in personal_access_tokens.
type PersonalAccessTokenID string

// PersonalAccessTokenScope is a coarse capability granted to a PAT.
type PersonalAccessTokenScope string

const (
	// PATScopeCatalogRead reads public catalog and product detail.
	PATScopeCatalogRead PersonalAccessTokenScope = "catalog:read"
	// PATScopeCartWrite manages the customer cart.
	PATScopeCartWrite PersonalAccessTokenScope = "cart:write"
	// PATScopeOrdersRead reads the customer's orders.
	PATScopeOrdersRead PersonalAccessTokenScope = "orders:read"
	// PATScopeOrdersWrite places orders / runs checkout on behalf of the customer.
	PATScopeOrdersWrite PersonalAccessTokenScope = "orders:write"
	// PATScopeSubscriptionsRead reads customer subscriptions.
	PATScopeSubscriptionsRead PersonalAccessTokenScope = "subscriptions:read"
	// PATScopeSubscriptionsWrite creates/updates/cancels customer subscriptions.
	PATScopeSubscriptionsWrite PersonalAccessTokenScope = "subscriptions:write"
	// PATScopeProfileRead reads customer profile and addresses.
	PATScopeProfileRead PersonalAccessTokenScope = "profile:read"
	// PATScopeProfileWrite updates customer profile and addresses.
	PATScopeProfileWrite PersonalAccessTokenScope = "profile:write"
)

// PersonalAccessToken is a hashed API credential bound to a customer.
// The raw secret is never persisted — only its SHA-256 hash and a short lookup prefix.
type PersonalAccessToken struct {
	ID          PersonalAccessTokenID
	CustomerID  CustomerID
	Name        string
	TokenPrefix string
	TokenHash   []byte
	Scopes      []PersonalAccessTokenScope
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// IsActive reports whether the token can still be used at t.
func (p PersonalAccessToken) IsActive(t time.Time) bool {
	if p.RevokedAt != nil {
		return false
	}
	if p.ExpiresAt != nil && !p.ExpiresAt.After(t) {
		return false
	}
	return true
}
