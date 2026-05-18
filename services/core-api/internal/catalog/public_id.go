package catalog

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"sip-happens/pkg/domain"
)

const publicIDPrefix = "prd_"

// PublicID returns the stable catalog identifier exposed in JSON (Terminal.shop-style).
func PublicID(id domain.ProductID) string {
	s := strings.TrimSpace(string(id))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, publicIDPrefix) {
		return s
	}
	if u, err := uuid.Parse(s); err == nil {
		return publicIDPrefix + strings.ReplaceAll(u.String(), "-", "")
	}
	// Opaque internal ids (tests): deterministic public id without DB migration.
	return publicIDPrefix + strings.ReplaceAll(s, "-", "")
}

// ProductIDFromPublicID decodes a prd_* value to the internal products.id when possible.
func ProductIDFromPublicID(pub string) (domain.ProductID, bool) {
	pub = strings.TrimSpace(pub)
	if !strings.HasPrefix(pub, publicIDPrefix) {
		return "", false
	}
	tail := strings.TrimPrefix(pub, publicIDPrefix)
	if len(tail) != 32 {
		return "", false
	}
	raw := tail[0:8] + "-" + tail[8:12] + "-" + tail[12:16] + "-" + tail[16:20] + "-" + tail[20:32]
	if _, err := uuid.Parse(raw); err != nil {
		return "", false
	}
	return domain.ProductID(raw), true
}

// ResolveProduct looks up a product by public id, internal UUID, SKU, or raw internal id.
func ResolveProduct(ctx context.Context, repo Repository, q Querier, ref string) (*domain.Product, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, domain.ErrInvalidInput
	}
	if id, ok := ProductIDFromPublicID(ref); ok {
		return repo.GetProduct(ctx, q, id)
	}
	if _, err := uuid.Parse(ref); err == nil {
		return repo.GetProduct(ctx, q, domain.ProductID(ref))
	}
	p, err := repo.GetProductBySKU(ctx, q, ref)
	if err == nil {
		return p, nil
	}
	if err != domain.ErrNotFound {
		return nil, err
	}
	return repo.GetProduct(ctx, q, domain.ProductID(ref))
}
