package http

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"sip-happens/pkg/auth"
)

const HeaderIdempotencyKey = "Idempotency-Key"

type IdempotencyStore interface {
	// Get returns (found, status, contentType, body).
	Get(ctx context.Context, scope, key, method, path, requestSHA string) (bool, int, string, []byte, error)
	// Put stores a successful response for later replay.
	Put(ctx context.Context, scope, key, method, path, requestSHA string, status int, contentType string, body []byte, expiresAt time.Time) error
}

var ErrIdempotencyKeyReuse = &idempotencyError{msg: "idempotency key reused with different request"}

type idempotencyError struct{ msg string }

func (e *idempotencyError) Error() string { return e.msg }

type SQLIdempotencyStore struct {
	DB *sql.DB
}

func (s *SQLIdempotencyStore) Get(ctx context.Context, scope, key, method, path, requestSHA string) (bool, int, string, []byte, error) {
	row := s.DB.QueryRowContext(ctx, `
select status_code, content_type, response_body, request_sha
from http_idempotency_keys
where scope=$1 and idem_key=$2 and method=$3 and path=$4 and expires_at > now()
`, scope, key, method, path)

	var status int
	var contentType sql.NullString
	var body []byte
	var storedSHA string
	if err := row.Scan(&status, &contentType, &body, &storedSHA); err != nil {
		if err == sql.ErrNoRows {
			return false, 0, "", nil, nil
		}
		return false, 0, "", nil, err
	}
	if storedSHA != requestSHA {
		return true, 0, "", nil, ErrIdempotencyKeyReuse
	}
	return true, status, contentType.String, body, nil
}

func (s *SQLIdempotencyStore) Put(ctx context.Context, scope, key, method, path, requestSHA string, status int, contentType string, body []byte, expiresAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
insert into http_idempotency_keys (scope, idem_key, method, path, request_sha, status_code, content_type, response_body, expires_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9)
on conflict (scope, idem_key, method, path) do nothing
`, scope, key, method, path, requestSHA, status, contentType, body, expiresAt)
	return err
}

type IdempotencyMiddleware struct {
	Store IdempotencyStore
	TTL   time.Duration
}

func (m *IdempotencyMiddleware) Wrap(next http.Handler) http.Handler {
	if m.TTL <= 0 {
		m.TTL = 24 * time.Hour
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		if m.Store == nil {
			writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
			return
		}

		key := strings.TrimSpace(r.Header.Get(HeaderIdempotencyKey))
		if key == "" {
			writeAPIError(w, r, http.StatusBadRequest, "missing_idempotency_key", "missing Idempotency-Key")
			return
		}

		scope := scopeFromRequest(r)
		path := r.URL.Path // do not include query string in idempotency keying

		bodyBytes, err := readAndRestoreBody(r)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid body")
			return
		}
		requestSHA := sha256Hex(bodyBytes)

		found, status, contentType, body, err := m.Store.Get(r.Context(), scope, key, r.Method, path, requestSHA)
		if err == ErrIdempotencyKeyReuse {
			writeAPIError(w, r, http.StatusConflict, "idempotency_key_reuse", "idempotency key reused with different request")
			return
		}
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if found && status != 0 {
			if strings.TrimSpace(contentType) != "" {
				w.Header().Set("content-type", contentType)
			}
			w.Header().Set("x-idempotency-replayed", "true")
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}

		rr := &recordingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rr, r)

		// Only persist successful responses.
		if rr.status != http.StatusOK && rr.status != http.StatusCreated && rr.status != http.StatusAccepted && rr.status != http.StatusNoContent {
			return
		}
		respBody := rr.body.Bytes()
		if len(respBody) > 1<<20 { // 1MB
			return
		}
		ct := rr.Header().Get("content-type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		_ = m.Store.Put(r.Context(), scope, key, r.Method, path, requestSHA, rr.status, ct, respBody, time.Now().Add(m.TTL))
	})
}

func scopeFromRequest(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/admin/") || r.URL.Path == "/admin/products" || r.URL.Path == "/admin/prices" || r.URL.Path == "/admin/inventory" {
		return "admin"
	}
	fp := strings.TrimSpace(r.Header.Get(auth.HeaderFingerprint))
	if fp != "" {
		return "fp:" + fp
	}
	return "anon"
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}

type recordingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *recordingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *recordingResponseWriter) Write(p []byte) (int, error) {
	_, _ = w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

