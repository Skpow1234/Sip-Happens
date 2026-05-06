package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	"sip-happens/pkg/auth"
)

type SigningRoundTripper struct {
	Next        http.RoundTripper
	Secret      []byte
	Fingerprint string
	Now         func() time.Time
}

func (rt SigningRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	next := rt.Next
	if next == nil {
		next = http.DefaultTransport
	}
	nowFn := rt.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		bodyBytes = b
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	ts := strconv.FormatInt(nowFn().Unix(), 10)
	nonce := randomHex(16)
	bodySHA := auth.BodySHA256Hex(bodyBytes)

	path := req.URL.RequestURI()
	canonical := auth.Canonical(req.Method, path, ts, nonce, bodySHA)
	sig := auth.Sign(rt.Secret, canonical)

	req2 := req.Clone(context.Background())
	req2.Header = req.Header.Clone()
	req2.Header.Set(auth.HeaderTimestamp, ts)
	req2.Header.Set(auth.HeaderNonce, nonce)
	req2.Header.Set(auth.HeaderSignature, sig)
	if rt.Fingerprint != "" {
		req2.Header.Set(auth.HeaderFingerprint, rt.Fingerprint)
	}

	// Restore body for cloned request.
	if bodyBytes != nil {
		req2.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	return next.RoundTrip(req2)
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

