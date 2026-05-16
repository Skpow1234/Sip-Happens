package seed

import (
	"strings"
	"testing"
)

func TestDemoSSHFingerprint_FromCommittedPubkey(t *testing.T) {
	fp, err := DemoSSHFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fp == DemoSSHFingerprintFallback {
		t.Fatalf("expected fingerprint from infra/demo/sip-happens-demo.pub, got fallback %q", fp)
	}
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("expected SHA256: prefix, got %q", fp)
	}
}
