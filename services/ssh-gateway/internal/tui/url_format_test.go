package tui

import (
	"strings"
	"testing"
)

func TestFormatSelectableURL_wraps(t *testing.T) {
	url := "https://checkout.stripe.com/pay/cs_test_" + strings.Repeat("x", 80)
	out := formatSelectableURL(url, 40)
	if !strings.Contains(out, "\n") {
		t.Fatalf("expected wrapped url, got %q", out)
	}
}
