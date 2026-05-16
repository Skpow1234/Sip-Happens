package redact

import (
	"strings"
	"testing"
)

func TestString_StripeSecretKey(t *testing.T) {
	in := "failed: sk_live_abc123xyz leaked"
	out := String(in)
	if strings.Contains(out, "abc123xyz") {
		t.Fatalf("secret not redacted: %q", out)
	}
	if !strings.Contains(out, placeholder) {
		t.Fatalf("expected placeholder in %q", out)
	}
}

func TestString_BearerToken(t *testing.T) {
	in := "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.sig"
	out := String(in)
	if strings.Contains(out, "eyJhbGci") {
		t.Fatalf("bearer not redacted: %q", out)
	}
}

func TestJSON_ClientSecret(t *testing.T) {
	in := `{"id":"cs_1","client_secret":"cs_test_secret_123","metadata":{"order_id":"ord-1"}}`
	out := JSON(in)
	if strings.Contains(out, "cs_test_secret_123") {
		t.Fatalf("client_secret not redacted: %q", out)
	}
	if !strings.Contains(out, placeholder) {
		t.Fatalf("expected placeholder in %q", out)
	}
	if !strings.Contains(out, "ord-1") {
		t.Fatalf("non-sensitive field removed: %q", out)
	}
}

func TestJSON_EmbeddedStripeKeyInString(t *testing.T) {
	in := `{"note":"key sk_test_abcdefghij was used"}`
	out := JSON(in)
	if strings.Contains(out, "abcdefghij") {
		t.Fatalf("embedded key not redacted: %q", out)
	}
}

func TestJSON_InvalidFallsBackToString(t *testing.T) {
	in := "not-json sk_live_deadbeef"
	out := JSON(in)
	if strings.Contains(out, "deadbeef") {
		t.Fatalf("expected redaction in opaque text: %q", out)
	}
}
