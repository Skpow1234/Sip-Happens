package security

import (
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
)

// SessionRestrictMiddleware blocks shell commands, subsystems, and non-PTY sessions.
// Port forwarding is disabled separately via SSHPortForwardingOptions.
func SessionRestrictMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if sub := strings.TrimSpace(s.Subsystem()); sub != "" {
				_, _ = wish.WriteString(s, "subsystems are disabled\r\n")
				_ = s.Exit(1)
				return
			}
			if cmd := strings.TrimSpace(s.RawCommand()); cmd != "" {
				_, _ = wish.WriteString(s, "remote commands are disabled; use the Sip-Happens TUI\r\n")
				_ = s.Exit(1)
				return
			}
			_, _, ok := s.Pty()
			if !ok {
				_, _ = wish.WriteString(s, "PTY required for the Sip-Happens TUI\r\n")
				_ = s.Exit(1)
				return
			}
			next(s)
		}
	}
}

// SSHPortForwardingOptions denies local and reverse SSH port forwarding.
func SSHPortForwardingOptions() []ssh.Option {
	return []ssh.Option{
		func(s *ssh.Server) error {
			s.LocalPortForwardingCallback = func(_ ssh.Context, _ string, _ uint32) bool { return false }
			s.ReversePortForwardingCallback = func(_ ssh.Context, _ string, _ uint32) bool { return false }
			return nil
		},
	}
}

// SSHDisablePasswordAuth rejects password authentication (public key / authorized_keys only).
func SSHDisablePasswordAuth() ssh.Option {
	return ssh.PasswordAuth(func(_ ssh.Context, _ string) bool { return false })
}
