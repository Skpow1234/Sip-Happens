package seed

import (
	"strings"
	"testing"
)

// TestDemoSSHFingerprint_FromCommittedPubkey verifies DemoSSHFingerprint
// reads the tracked demo public key (infra/demo/sip-happens-demo.pub).
func TestDemoSSHFingerprint_FromCommittedPubkey(t *testing.T) {
	fp, err := DemoSSHFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fp == DemoSSHFingerprintFallback {
		t.Fatal("infra/demo/sip-happens-demo.pub missing; demo pubkey must be tracked (M1)")
	}
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("expected SHA256: prefix, got %q", fp)
	}
	const want = "SHA256:kqjoLaSTFR7d2Vg62ebJEeM6hwJCbJcLzqvGnBxFQp8"
	if fp != want {
		t.Fatalf("demo fingerprint drift: got %q want %q (update Compose SSH_DEV_FINGERPRINT)", fp, want)
	}
}
