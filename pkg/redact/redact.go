package redact

import (
	"encoding/json"
	"regexp"
	"strings"
)

const placeholder = "[REDACTED]"

var bearerRE = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-]+`)

// Stripe-style secret tokens embedded in text.
var stripeTokenRE = regexp.MustCompile(`(?:sk|rk)_(?:live|test)_[A-Za-z0-9]+|whsec_[A-Za-z0-9]+|pk_(?:live|test)_[A-Za-z0-9]+`)

// Sensitive JSON object keys (lowercase match).
var sensitiveKeys = map[string]struct{}{
	"client_secret":          {},
	"secret":                 {},
	"api_key":                {},
	"private_key":            {},
	"password":               {},
	"authorization":          {},
	"stripe_signature":       {},
	"webhook_secret":         {},
	"signing_secret":         {},
	"restricted_key":         {},
	"publishable_key":        {}, // often pk_*; value also scanned
	"stripe_secret_key":      {},
	"stripe_webhook_secret":  {},
	"admin_token":            {},
	"access_token":           {},
	"refresh_token":          {},
}

// String replaces known secret patterns in free text (logs, error messages).
func String(s string) string {
	if s == "" {
		return s
	}
	out := bearerRE.ReplaceAllString(s, "Bearer "+placeholder)
	return stripeTokenRE.ReplaceAllString(out, placeholder)
}

// JSON redacts sensitive keys and secret-like substrings in a JSON document.
// Invalid JSON is treated as opaque text via String.
func JSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return String(s)
	}
	sanitizeValue(&v)
	b, err := json.Marshal(v)
	if err != nil {
		return String(s)
	}
	return string(b)
}

func sanitizeValue(v *any) {
	switch t := (*v).(type) {
	case map[string]any:
		for k, val := range t {
			if isSensitiveKey(k) {
				t[k] = placeholder
				continue
			}
			sanitizeValue(&val)
			t[k] = val
		}
	case []any:
		for i := range t {
			sanitizeValue(&t[i])
		}
	case string:
		*v = String(t)
	}
}

func isSensitiveKey(k string) bool {
	_, ok := sensitiveKeys[strings.ToLower(strings.TrimSpace(k))]
	return ok
}

// Fields returns a copy of m with string values and nested maps/lists sanitized for logging.
func Fields(m map[string]any) map[string]any {
	if len(m) == 0 {
		return m
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = sanitizeFieldValue(v)
	}
	return out
}

func sanitizeFieldValue(v any) any {
	switch t := v.(type) {
	case string:
		return String(t)
	case map[string]any:
		var anyV any = t
		sanitizeValue(&anyV)
		return anyV
	case []any:
		cp := make([]any, len(t))
		copy(cp, t)
		for i := range cp {
			cp[i] = sanitizeFieldValue(cp[i])
		}
		return cp
	default:
		return v
	}
}
