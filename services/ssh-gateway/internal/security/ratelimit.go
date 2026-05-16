package security

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/ssh"

	"sip-happens/services/ssh-gateway/internal/observability"
)

// SSHRateLimiter limits new SSH sessions per window by remote IP and SSH key fingerprint.
type SSHRateLimiter struct {
	Window time.Duration
	Limit  int

	mu   sync.Mutex
	hits map[string][]time.Time
}

func (l *SSHRateLimiter) Middleware() func(ssh.Handler) ssh.Handler {
	if l.Window <= 0 {
		l.Window = time.Minute
	}
	if l.Limit <= 0 {
		l.Limit = 30
	}
	if l.hits == nil {
		l.hits = map[string][]time.Time{}
	}

	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			keys := rateLimitKeys(s)
			if len(keys) == 0 {
				next(s)
				return
			}

			now := time.Now()
			for _, key := range keys {
				if !l.allow(key, now) {
					observability.RecordRateLimited()
					_, _ = s.Write([]byte("Too many attempts. Try again later.\n"))
					_ = s.Exit(1)
					return
				}
			}

			observability.RecordConnection()
			next(s)
		}
	}
}

func rateLimitKeys(s ssh.Session) []string {
	ip := remoteIP(s.RemoteAddr().String())
	fp := SessionFingerprint(s)
	var keys []string
	if ip != "" {
		keys = append(keys, "ip:"+ip)
	}
	if fp != "" {
		keys = append(keys, "fp:"+strings.ToLower(fp))
	}
	return keys
}

func (l *SSHRateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.Window)
	list := l.hits[key]
	dst := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			dst = append(dst, t)
		}
	}
	list = dst
	if len(list) >= l.Limit {
		l.hits[key] = list
		return false
	}
	list = append(list, now)
	l.hits[key] = list
	return true
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil && host != "" {
		return host
	}
	return ""
}
