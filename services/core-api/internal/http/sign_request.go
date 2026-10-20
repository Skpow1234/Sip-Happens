package http

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"

	"sip-happens/pkg/auth"
)

// SignRequest adds HMAC v2 headers for integration tests and e2e smoke runs.
// Each call uses a fresh nonce/request-id so multi-step contract tests do not
// trip the SQL nonce replay store within the same Unix second.
func SignRequest(req *http.Request, secret []byte, keyID, fingerprint string) error {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		_ = req.Body.Close()
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := randomHex(16)
	if err != nil {
		return err
	}
	reqID, err := randomHex(8)
	if err != nil {
		return err
	}
	bodySHA := auth.BodySHA256Hex(body)
	path := req.URL.RequestURI()
	canonical := auth.CanonicalV2(req.Method, path, ts, nonce, reqID, bodySHA)
	sig := auth.Sign(secret, canonical)

	req.Header.Set(auth.HeaderTimestamp, ts)
	req.Header.Set(auth.HeaderNonce, nonce)
	req.Header.Set(auth.HeaderSignature, sig)
	req.Header.Set(auth.HeaderKeyID, keyID)
	req.Header.Set(auth.HeaderRequestID, reqID)
	req.Header.Set(auth.HeaderFingerprint, fingerprint)
	return nil
}

func randomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
