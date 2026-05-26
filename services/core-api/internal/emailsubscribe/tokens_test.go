package emailsubscribe

import (
	"strings"
	"testing"
)

// TestGenerateConfirmationToken_Shape pins the visible token format
// (`subc_<32-hex-prefix>_<43-char-base64-secret>`). The TUI / email
// link build is sensitive to this shape, so an accidental rename would
// silently break confirmation links — this test fails loudly.
func TestGenerateConfirmationToken_Shape(t *testing.T) {
	raw, prefix, hash, err := generateConfirmationToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(raw, "subc_") {
		t.Fatalf("raw should start with subc_, got %q", raw)
	}
	body := strings.TrimPrefix(raw, "subc_")
	parts := strings.SplitN(body, "_", 2)
	if len(parts) != 2 {
		t.Fatalf("raw not in <prefix>_<secret> form: %q", raw)
	}
	if got, want := len(parts[0]), confirmPrefixBytes*2; got != want {
		t.Fatalf("prefix length: got %d want %d", got, want)
	}
	if parts[0] != prefix {
		t.Fatalf("returned prefix mismatch: %q vs %q", parts[0], prefix)
	}
	// 32-byte secret -> 43-char base64url (no padding).
	if got, want := len(parts[1]), 43; got != want {
		t.Fatalf("secret length: got %d want %d", got, want)
	}
	if len(hash) != 32 {
		t.Fatalf("hash length should be 32 bytes, got %d", len(hash))
	}
}

func TestParseConfirmationToken_RoundTrip(t *testing.T) {
	raw, want, _, err := generateConfirmationToken()
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseConfirmationToken(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != want {
		t.Fatalf("prefix: got %q want %q", got, want)
	}
}

func TestParseConfirmationToken_Rejects(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"wrong-label", "pat_0123456789abcdef0123456789abcdef_secretsecretsecretsecretsecretsecretsecret"},
		{"no-underscore", "subc_0123456789abcdef0123456789abcdef"},
		{"short-prefix", "subc_dead_secret"},
		{"non-hex-prefix", "subc_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz_secret"},
		{"empty-secret", "subc_0123456789abcdef0123456789abcdef_"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseConfirmationToken(tc.raw); err == nil {
				t.Fatalf("expected error for %q", tc.raw)
			}
		})
	}
}

func TestVerifyConfirmationToken_ConstantTime(t *testing.T) {
	raw, _, hash, err := generateConfirmationToken()
	if err != nil {
		t.Fatal(err)
	}
	if !verifyConfirmationToken(raw, hash) {
		t.Fatal("verify with matching hash should succeed")
	}
	// Flipping a single byte should make verification fail.
	hash[0] ^= 0xFF
	if verifyConfirmationToken(raw, hash) {
		t.Fatal("verify with corrupted hash should fail")
	}
	// Wrong-length stored hash must fail (defense against truncation bugs).
	if verifyConfirmationToken(raw, hash[:16]) {
		t.Fatal("verify with truncated stored hash should fail")
	}
}
