package tui

import "testing"

func TestASCIQR_nonEmpty(t *testing.T) {
	out := asciiQR("https://checkout.stripe.com/pay/cs_test", 120)
	if out == "" {
		t.Fatal("expected QR output")
	}
	if !contains(out, "██") {
		t.Fatalf("expected block chars, got %q", out[:min(40, len(out))])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
