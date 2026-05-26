package api

import (
	"net/http"
	"strings"
)

// BearerRoundTripper authenticates outgoing requests with a
// `Authorization: Bearer <pat>` header instead of HMAC. Used by
// programmatic clients that hold a personal access token (PAT) rather
// than an SSH-derived HMAC secret. The PAT is the same string the user
// got from PersonalAccessTokenCreateResult.Token; the core-api validates
// it through the same tokens.SQLRepository the /tokens CRUD writes to.
//
// X-SSH-Fingerprint is still set when provided so audit rows keep the
// "who" attribution; core-api treats the PAT as the source of truth for
// authentication and the fingerprint as a tag.
//
// The X-Request-Id header is auto-generated when missing for correlation.
type BearerRoundTripper struct {
	Next        http.RoundTripper
	Token       string
	Fingerprint string
}

func (rt BearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	next := rt.Next
	if next == nil {
		next = http.DefaultTransport
	}
	req2 := req.Clone(req.Context())
	req2.Header = req.Header.Clone()
	if t := strings.TrimSpace(rt.Token); t != "" {
		req2.Header.Set("Authorization", "Bearer "+t)
	}
	if fp := strings.TrimSpace(rt.Fingerprint); fp != "" {
		req2.Header.Set("X-SSH-Fingerprint", fp)
	}
	if strings.TrimSpace(req2.Header.Get("X-Request-Id")) == "" {
		req2.Header.Set("X-Request-Id", randomHex(16))
	}
	return next.RoundTrip(req2)
}
