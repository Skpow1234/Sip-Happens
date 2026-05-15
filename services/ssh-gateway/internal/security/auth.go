package security

import (
	"log"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"

	"sip-happens/pkg/sshfp"
)

// SSHOptions returns Wish/SSH server options for the configured auth mode.
func SSHOptions(requireAuthorizedKeys bool, authorizedKeysPath string) []ssh.Option {
	if requireAuthorizedKeys {
		if authorizedKeysPath == "" {
			log.Fatal("SSH_AUTHORIZED_KEYS_PATH is required when SSH_REQUIRE_AUTHORIZED_KEYS=1")
		}
		return []ssh.Option{
			wish.WithAuthorizedKeys(authorizedKeysPath),
		}
	}
	// Dev: accept any public-key auth so `ssh -p 2222 localhost` works without a keys file.
	return []ssh.Option{
		ssh.PublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool { return true }),
	}
}

// SessionFingerprint returns the OpenSSH-compatible SHA256 fingerprint for the session's public key.
func SessionFingerprint(s ssh.Session) string {
	pk := s.PublicKey()
	if pk == nil {
		return ""
	}
	return sshfp.FromMarshaled(pk.Marshal())
}
