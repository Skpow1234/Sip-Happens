package addresses

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestPublicID_UUIDRoundTrip(t *testing.T) {
	internal := domain.AddressID("a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	pub := PublicID(internal)
	id, ok := AddressIDFromPublicID(pub)
	if !ok || id != internal {
		t.Fatalf("round trip: pub=%q id=%q ok=%v", pub, id, ok)
	}
}
