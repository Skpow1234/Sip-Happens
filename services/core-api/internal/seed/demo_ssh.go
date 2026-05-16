package seed

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/crypto/ssh"

	"sip-happens/pkg/sshfp"
)

// DemoSSHFingerprintFallback is used when the demo .pub file is not found (e.g. tests).
const DemoSSHFingerprintFallback = "demo:ssh-fingerprint"

// DemoSSHFingerprint returns the OpenSSH SHA256 fingerprint for the committed demo public key.
func DemoSSHFingerprint() (string, error) {
	b, err := loadDemoPublicKeyBytes()
	if err != nil {
		return DemoSSHFingerprintFallback, nil
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey(b)
	if err != nil {
		return "", fmt.Errorf("parse demo public key: %w", err)
	}
	return sshfp.PublicKeyFingerprint(pk), nil
}

func loadDemoPublicKeyBytes() ([]byte, error) {
	for _, p := range demoPublicKeyPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		b = bytes.TrimSpace(b)
		if len(b) > 0 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("demo public key not found")
}

func demoPublicKeyPaths() []string {
	var out []string
	if v := os.Getenv("DEMO_SSH_PUBKEY_PATH"); v != "" {
		out = append(out, v)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		// .../services/core-api/internal/seed/demo_ssh.go -> repo root
		root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
		out = append(out, filepath.Join(root, "infra", "demo", "sip-happens-demo.pub"))
	}
	out = append(out,
		"infra/demo/sip-happens-demo.pub",
		"/app/infra/demo/sip-happens-demo.pub",
	)
	return out
}
