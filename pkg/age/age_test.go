package age

import (
	"testing"
	"time"
)

func TestIsAdult(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	adult := time.Date(2000, 5, 14, 0, 0, 0, 0, time.UTC)
	minor := time.Date(2010, 5, 15, 0, 0, 0, 0, time.UTC)
	if !IsAdult(adult, now) {
		t.Fatal("expected adult")
	}
	if IsAdult(minor, now) {
		t.Fatal("expected minor")
	}
}
