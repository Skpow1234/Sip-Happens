package http

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"sip-happens/pkg/auth"
)

// RateLimiter is implemented by in-memory and Redis token bucket limiters.
type RateLimiter interface {
	Allow(key string, now time.Time) bool
}

type TokenBucketLimiter struct {
	Capacity   float64
	RefillRate float64 // tokens per second
	TTL        time.Duration

	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens  float64
	last    time.Time
	expires time.Time
}

func (l *TokenBucketLimiter) Allow(key string, now time.Time) bool {
	if l.Capacity <= 0 || l.RefillRate <= 0 {
		return true
	}
	if l.TTL <= 0 {
		l.TTL = 10 * time.Minute
	}
	if l.buckets == nil {
		l.buckets = map[string]*tokenBucket{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Lazy cleanup.
	for k, b := range l.buckets {
		if now.After(b.expires) {
			delete(l.buckets, k)
		}
	}

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &tokenBucket{
			tokens:  l.Capacity - 1,
			last:    now,
			expires: now.Add(l.TTL),
		}
		return true
	}

	dt := now.Sub(b.last).Seconds()
	if dt > 0 {
		b.tokens = minF(l.Capacity, b.tokens+dt*l.RefillRate)
		b.last = now
	}
	b.expires = now.Add(l.TTL)

	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

type RateLimitMiddleware struct {
	Limiter RateLimiter
	KeyFn   func(r *http.Request) string
}

func (m *RateLimitMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.Limiter == nil || m.KeyFn == nil {
			next.ServeHTTP(w, r)
			return
		}
		key := m.KeyFn(r)
		if strings.TrimSpace(key) == "" {
			key = "anon"
		}
		if !m.Limiter.Allow(key, time.Now()) {
			writeAPIError(w, r, http.StatusTooManyRequests, "rate_limited", "rate limited")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func KeyByFingerprintOrIP(r *http.Request) string {
	if fp := strings.TrimSpace(r.Header.Get(auth.HeaderFingerprint)); fp != "" {
		return "fp:" + fp
	}
	return "ip:" + rateLimitClientIP(r)
}

func KeyByAdminTokenOrIP(r *http.Request) string {
	// Do not store the raw token in-memory; hash it.
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(h, "Bearer ") {
		tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if tok != "" {
			sum := sha256.Sum256([]byte(tok))
			return "adm:" + hex.EncodeToString(sum[:])
		}
	}
	return "admip:" + rateLimitClientIP(r)
}

func rateLimitClientIP(r *http.Request) string {
	// Best effort: if behind a proxy, prefer first X-Forwarded-For.
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
