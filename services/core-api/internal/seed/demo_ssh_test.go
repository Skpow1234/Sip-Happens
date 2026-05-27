package seed

import (
	"strings"
	"testing"
)

// TestDemoSSHFingerprint_FromCommittedPubkey verifies that, when the
// demo public key is present on disk, DemoSSHFingerprint produces a real
// SHA256 fingerprint (not the synthetic "demo:ssh-fingerprint" fallback
// the seeder uses in environments without the file).
//
// `infra/demo/` is intentionally excluded by `.gitignore`, so on fresh
// CI checkouts the file is absent. In that case there's nothing to
// verify and the test skips, matching how `openapi_drift_test.go` and
// `scripts/check-migration-linearity.sh` handle their gitignored
// dependencies.
func TestDemoSSHFingerprint_FromCommittedPubkey(t *testing.T) {
	fp, err := DemoSSHFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fp == DemoSSHFingerprintFallback {
		t.Skipf("infra/demo/sip-happens-demo.pub not present; skipping demo-pubkey fingerprint check")
		return
	}
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("expected SHA256: prefix, got %q", fp)
	}
}
