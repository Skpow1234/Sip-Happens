package http

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"strconv"
	"time"

	"sip-happens/pkg/auth"
)

type NonceStore interface {
	// Remember returns true if the (fingerprint, nonce, ts) was already seen.
	Remember(ctx context.Context, fingerprint, nonce string, ts int64, expiresAt time.Time) (alreadySeen bool, err error)
}

type SQLNonceStore struct {
	DB *sql.DB
}

func (s *SQLNonceStore) Remember(ctx context.Context, fingerprint, nonce string, ts int64, expiresAt time.Time) (bool, error) {
	if s == nil || s.DB == nil {
		return false, sql.ErrConnDone
	}
	// Best-effort cleanup. Keeps table bounded without a dedicated janitor job (for now).
	_, _ = s.DB.ExecContext(ctx, `delete from hmac_nonces where expires_at < now()`)

	res, err := s.DB.ExecContext(ctx, `
insert into hmac_nonces (fingerprint, nonce, ts, expires_at)
values ($1, $2, $3, $4)
on conflict (fingerprint, nonce, ts) do nothing
`, fingerprint, nonce, ts, expiresAt)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

type HMACMiddleware struct {
	// LegacySecret is the v1 secret (no key-id, no request-id binding).
	LegacySecret []byte
	// Secrets is a v2 key ring, looked up by X-SipHappens-Key-Id.
	Secrets   map[string][]byte
	Tolerance time.Duration
	Nonces   NonceStore
}

func (m *HMACMiddleware) Wrap(next http.Handler) http.Handler {
	if m.Tolerance <= 0 {
		m.Tolerance = 5 * time.Minute
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(m.LegacySecret) == 0 && len(m.Secrets) == 0 {
			writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
			return
		}

		ts := r.Header.Get(auth.HeaderTimestamp)
		nonce := r.Header.Get(auth.HeaderNonce)
		sig := r.Header.Get(auth.HeaderSignature)
		kid := r.Header.Get(auth.HeaderKeyID)
		reqID := r.Header.Get(auth.HeaderRequestID)
		fp := r.Header.Get(auth.HeaderFingerprint)

		if err := auth.ParseUnixSeconds(ts, time.Now(), m.Tolerance); err != nil {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		if nonce == "" {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		if fp == "" {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "missing fingerprint")
			return
		}
		if m.Nonces == nil {
			writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
			return
		}
		sec, err := parseInt64(ts)
		if err != nil {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		alreadySeen, err := m.Nonces.Remember(r.Context(), fp, nonce, sec, time.Now().Add(m.Tolerance))
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if alreadySeen {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "replay detected")
			return
		}

		var bodyBytes []byte
		if r.Body != nil {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid body")
				return
			}
			_ = r.Body.Close()
			bodyBytes = b
		}
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		bodySHA := auth.BodySHA256Hex(bodyBytes)
		path := r.URL.RequestURI()
		var canonical string
		var secret []byte
		if kid != "" && len(m.Secrets) > 0 {
			secret = m.Secrets[kid]
			canonical = auth.CanonicalV2(r.Method, path, ts, nonce, reqID, bodySHA)
		} else {
			secret = m.LegacySecret
			canonical = auth.Canonical(r.Method, path, ts, nonce, bodySHA)
		}
		if len(secret) == 0 {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
		if err := auth.Verify(secret, canonical, sig); err != nil {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

