package tui

import (
	"testing"
	"time"
)

func TestFormatExpiryCountdown(t *testing.T) {
	now := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	exp := now.Add(2*time.Minute + 5*time.Second)
	got := formatExpiryCountdown(&exp, now)
	if got != "02:05" {
		t.Fatalf("got %q", got)
	}
	past := now.Add(-time.Second)
	if formatExpiryCountdown(&past, now) != "expired" {
		t.Fatal("expected expired")
	}
	if formatExpiryCountdown(nil, now) != "" {
		t.Fatal("expected empty")
	}
}
