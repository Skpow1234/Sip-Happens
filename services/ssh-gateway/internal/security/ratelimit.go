package security

import (
	"net"
	"sync"
	"time"

	"github.com/charmbracelet/ssh"
)

// Simple in-memory IP rate limiter for SSH sessions.
// Allows N new sessions per window per remote IP.
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
			ip := remoteIP(s.RemoteAddr().String())
			if ip == "" {
				next(s)
				return
			}

			now := time.Now()
			if !l.allow(ip, now) {
				_, _ = s.Write([]byte("Too many attempts. Try again later.\n"))
				_ = s.Exit(1)
				return
			}

			next(s)
		}
	}
}

func (l *SSHRateLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.Window)
	list := l.hits[ip]
	dst := list[:0]
	for _, t := range list {
		if t.After(cutoff) {
			dst = append(dst, t)
		}
	}
	list = dst
	if len(list) >= l.Limit {
		l.hits[ip] = list
		return false
	}
	list = append(list, now)
	l.hits[ip] = list
	return true
}

func remoteIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil && host != "" {
		return host
	}
	return ""
}

