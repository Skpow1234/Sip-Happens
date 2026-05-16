package http

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"sip-happens/pkg/auth"
)

// SignRequest adds HMAC v2 headers for integration tests and e2e smoke runs.
func SignRequest(req *http.Request, secret []byte, keyID, fingerprint string) error {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(body))

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "e2enonce1234567890"
	reqID := "e2e-req-1"
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
