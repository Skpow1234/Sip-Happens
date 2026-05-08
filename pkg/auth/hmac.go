package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	HeaderTimestamp   = "X-SipHappens-Timestamp"
	HeaderNonce       = "X-SipHappens-Nonce"
	HeaderSignature   = "X-SipHappens-Signature"
	HeaderFingerprint = "X-SSH-Fingerprint"
	HeaderKeyID       = "X-SipHappens-Key-Id"
	HeaderRequestID   = "X-Request-Id"
)

func BodySHA256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func Canonical(method, path, ts, nonce, bodySHA string) string {
	// NOTE: path must include query string if present.
	return strings.ToUpper(method) + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + bodySHA
}

func CanonicalV2(method, path, ts, nonce, requestID, bodySHA string) string {
	// NOTE: request_id is included to bind signatures to a correlation id and
	// make replay across requests harder to disguise.
	// NOTE: path must include query string if present.
	return strings.ToUpper(method) + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + requestID + "\n" + bodySHA
}

func Sign(secret []byte, canonical string) string {
	m := hmac.New(sha256.New, secret)
	_, _ = m.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func Verify(secret []byte, canonical, sigB64 string) error {
	if sigB64 == "" {
		return errors.New("missing signature")
	}
	got, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	m := hmac.New(sha256.New, secret)
	_, _ = m.Write([]byte(canonical))
	want := m.Sum(nil)
	if !hmac.Equal(got, want) {
		return errors.New("signature mismatch")
	}
	return nil
}

func ParseUnixSeconds(ts string, now time.Time, tolerance time.Duration) error {
	if ts == "" {
		return errors.New("missing timestamp")
	}
	sec, err := parseInt64(ts)
	if err != nil {
		return errors.New("invalid timestamp")
	}
	t := time.Unix(sec, 0)
	if t.Before(now.Add(-tolerance)) || t.After(now.Add(tolerance)) {
		return errors.New("timestamp outside tolerance")
	}
	return nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	if s == "" {
		return 0, errors.New("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("non-digit")
		}
		n = n*10 + int64(r-'0')
	}
	return n, nil
}

