package sshfp

import (
	gossh "golang.org/x/crypto/ssh"
)

// PublicKeyFingerprint returns a stable OpenSSH-compatible fingerprint (SHA256:…)
// for the given SSH public key. This value is stored as customers.ssh_fingerprint.
func PublicKeyFingerprint(pk gossh.PublicKey) string {
	if pk == nil {
		return ""
	}
	return gossh.FingerprintSHA256(pk)
}

// FromMarshaled parses key bytes and returns the fingerprint, or empty on parse error.
func FromMarshaled(m []byte) string {
	if len(m) == 0 {
		return ""
	}
	pk, err := gossh.ParsePublicKey(m)
	if err != nil {
		return ""
	}
	return gossh.FingerprintSHA256(pk)
}
