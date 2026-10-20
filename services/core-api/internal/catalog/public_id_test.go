package catalog

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestPublicID_UUIDRoundTrip(t *testing.T) {
	internal := domain.ProductID("550e8400-e29b-41d4-a716-446655440000")
	pub := PublicID(internal)
	if pub != "prd_550e8400e29b41d4a716446655440000" {
		t.Fatalf("public id: %q", pub)
	}
	id, ok := ProductIDFromPublicID(pub)
	if !ok || id != internal {
		t.Fatalf("decode: ok=%v id=%q", ok, id)
	}
}

func TestPublicID_OpaqueInternalRoundTrip(t *testing.T) {
	internal := domain.ProductID("p_test_1")
	pub := PublicID(internal)
	if pub != "prd_p_test_1" {
		t.Fatalf("public id: %q", pub)
	}
	id, ok := ProductIDFromPublicID(pub)
	if !ok || id != internal {
		t.Fatalf("opaque decode: ok=%v id=%q want %q", ok, id, internal)
	}
}

func TestProductIDFromPublicID_Invalid(t *testing.T) {
	if _, ok := ProductIDFromPublicID("SKU-1"); ok {
		t.Fatal("expected false for sku")
	}
	if _, ok := ProductIDFromPublicID("prd_"); ok {
		t.Fatal("expected false for empty tail")
	}
	if _, ok := ProductIDFromPublicID(""); ok {
		t.Fatal("expected false for empty")
	}
}
