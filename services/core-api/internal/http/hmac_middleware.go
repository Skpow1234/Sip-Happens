package http

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"sip-happens/pkg/auth"
)

type HMACMiddleware struct {
	Secret    []byte
	Tolerance time.Duration

	mu     sync.Mutex
	nonces map[string]time.Time
}

func (m *HMACMiddleware) Wrap(next http.Handler) http.Handler {
	if m.nonces == nil {
		m.nonces = map[string]time.Time{}
	}
	if m.Tolerance <= 0 {
		m.Tolerance = 5 * time.Minute
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(m.Secret) == 0 {
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		ts := r.Header.Get(auth.HeaderTimestamp)
		nonce := r.Header.Get(auth.HeaderNonce)
		sig := r.Header.Get(auth.HeaderSignature)

		if err := auth.ParseUnixSeconds(ts, time.Now(), m.Tolerance); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if nonce == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if m.seen(nonce, ts) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var bodyBytes []byte
		if r.Body != nil {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			_ = r.Body.Close()
			bodyBytes = b
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		bodySHA := auth.BodySHA256Hex(bodyBytes)
		path := r.URL.RequestURI()
		canonical := auth.Canonical(r.Method, path, ts, nonce, bodySHA)
		if err := auth.Verify(m.Secret, canonical, sig); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *HMACMiddleware) seen(nonce, ts string) bool {
	sec, err := parseInt64(ts)
	if err != nil {
		return true
	}
	t := time.Unix(sec, 0)
	key := nonce + ":" + ts

	m.mu.Lock()
	defer m.mu.Unlock()

	// GC.
	cutoff := time.Now().Add(-m.Tolerance)
	for k, v := range m.nonces {
		if v.Before(cutoff) {
			delete(m.nonces, k)
		}
	}

	if _, ok := m.nonces[key]; ok {
		return true
	}
	m.nonces[key] = t
	return false
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

