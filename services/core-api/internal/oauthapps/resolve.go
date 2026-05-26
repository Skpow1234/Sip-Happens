package oauthapps

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"sip-happens/pkg/domain"
)

// ResolveClient maps a path-segment reference (either an `oc_<32 hex>` public
// id or the bare internal UUID) to a domain.OAuthClient scoped to the caller's
// customer. Returns domain.ErrNotFound when the reference is malformed or
// belongs to another customer.
func ResolveClient(ctx context.Context, repo ClientRepository, q Querier, customerID domain.CustomerID, ref string) (*domain.OAuthClient, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, domain.ErrNotFound
	}
	if id, ok := IDFromPublicID(ref); ok {
		return repo.Get(ctx, q, customerID, id)
	}
	if _, err := uuid.Parse(ref); err == nil {
		return repo.Get(ctx, q, customerID, domain.OAuthClientID(ref))
	}
	return nil, domain.ErrNotFound
}
