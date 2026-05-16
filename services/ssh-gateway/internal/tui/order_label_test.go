package tui

import "testing"

func TestFormatOrderLabel_PrefersDisplayRef(t *testing.T) {
	if got := formatOrderLabel("uuid-long-id", "SH-ABCD1234"); got != "SH-ABCD1234" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatOrderLabel_FallsBackToID(t *testing.T) {
	if got := formatOrderLabel("short", ""); got != "short" {
		t.Fatalf("got %q", got)
	}
}
