package orders

import "testing"

func TestRefLooksLikeDisplayRef(t *testing.T) {
	if !RefLooksLikeDisplayRef("SH-ABC12345") {
		t.Fatal("expected display ref")
	}
	if RefLooksLikeDisplayRef("order-uuid") {
		t.Fatal("expected false for uuid")
	}
}
