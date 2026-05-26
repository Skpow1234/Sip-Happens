package domain

import "testing"

func TestInventoryAvailable(t *testing.T) {
	i := Inventory{OnHand: 10, Reserved: 3, SafetyStock: 2}
	if got := i.Available(); got != 5 {
		t.Fatalf("expected available=5, got=%d", got)
	}
}
