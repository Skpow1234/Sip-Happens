package oauthapps

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// Credential formats issued to third-party apps and exchange callers:
//
//   client_id              — public, embedded in third-party config:
//                            oci_<32-hex>            (32-hex stable handle)
//
//   client_secret          — confidential, shown ONCE at registration:
//                            ocs_<32-hex-prefix>_<43-char-base64url-secret>
//
//   authorization code     — short-lived, single-use; shown ONCE by /oauth/authorize:
//                            oac_<32-hex-prefix>_<43-char-base64url-secret>
//
// Lookups always hit token_prefix / client_secret_prefix / code_prefix indexes
// and only then constant-time compare the SHA-256 of the full raw string.

const (
	clientIDPrefix     = "oci_"
	clientSecretPrefix = "ocs_"
	authzCodePrefix    = "oac_"

	prefixBytes = 16 // 32 hex chars
	secretBytes = 32 // 43 base64url chars (no padding) → ~256 bits
)

// ErrInvalidRawSecret indicates the provided client secret or auth code did
// not match the expected `prefix_<32hex>_<base64url>` shape.
var ErrInvalidRawSecret = errors.New("oauthapps: invalid raw secret format")

// GeneratedClientID is the value persisted in oauth_clients.client_id (text
// column). Stored verbatim, indexed for /oauth/token lookups.
func GenerateClientID() (string, error) {
	buf := make([]byte, prefixBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return clientIDPrefix + hex.EncodeToString(buf), nil
}

// GeneratedClientSecret bundles the user-visible raw secret with the values
// the server persists (Prefix + Hash). Raw must be shown to the user exactly
// once, then discarded.
type GeneratedClientSecret struct {
	Raw    string
	Prefix string
	Hash   []byte
}

// GenerateClientSecret produces a fresh confidential client secret using
// crypto/rand. The Raw value is what the third-party app must keep safe;
// the caller stores Prefix + Hash and discards Raw.
func GenerateClientSecret() (GeneratedClientSecret, error) {
	prefixBuf := make([]byte, prefixBytes)
	if _, err := rand.Read(prefixBuf); err != nil {
		return GeneratedClientSecret{}, err
	}
	secretBuf := make([]byte, secretBytes)
	if _, err := rand.Read(secretBuf); err != nil {
		return GeneratedClientSecret{}, err
	}
	prefix := hex.EncodeToString(prefixBuf)
	secret := base64.RawURLEncoding.EncodeToString(secretBuf)
	raw := clientSecretPrefix + prefix + "_" + secret
	sum := sha256.Sum256([]byte(raw))
	return GeneratedClientSecret{
		Raw:    raw,
		Prefix: prefix,
		Hash:   sum[:],
	}, nil
}

// GeneratedAuthorizationCode is the analogue for /oauth/authorize.
type GeneratedAuthorizationCode struct {
	Raw    string
	Prefix string
	Hash   []byte
}

// GenerateAuthorizationCode mints a fresh single-use code. Same format as
// the client secret but with the `oac_` label so logs and auth code can
// route the two shapes without ambiguity.
func GenerateAuthorizationCode() (GeneratedAuthorizationCode, error) {
	prefixBuf := make([]byte, prefixBytes)
	if _, err := rand.Read(prefixBuf); err != nil {
		return GeneratedAuthorizationCode{}, err
	}
	secretBuf := make([]byte, secretBytes)
	if _, err := rand.Read(secretBuf); err != nil {
		return GeneratedAuthorizationCode{}, err
	}
	prefix := hex.EncodeToString(prefixBuf)
	secret := base64.RawURLEncoding.EncodeToString(secretBuf)
	raw := authzCodePrefix + prefix + "_" + secret
	sum := sha256.Sum256([]byte(raw))
	return GeneratedAuthorizationCode{
		Raw:    raw,
		Prefix: prefix,
		Hash:   sum[:],
	}, nil
}

// parsePrefixed extracts the lookup prefix from a `<label>_<prefix>_<secret>`
// string and validates the visible structure. Used by both client-secret
// verification (label="ocs") and authorization-code redemption (label="oac").
func parsePrefixed(label, raw string) (prefix string, err error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, label) {
		return "", ErrInvalidRawSecret
	}
	body := strings.TrimPrefix(raw, label)
	parts := strings.SplitN(body, "_", 2)
	if len(parts) != 2 {
		return "", ErrInvalidRawSecret
	}
	prefix = parts[0]
	if len(prefix) != prefixBytes*2 {
		return "", ErrInvalidRawSecret
	}
	if _, err := hex.DecodeString(prefix); err != nil {
		return "", ErrInvalidRawSecret
	}
	if len(parts[1]) == 0 {
		return "", ErrInvalidRawSecret
	}
	return prefix, nil
}

// ParseClientSecret returns the lookup prefix from an `ocs_…` string.
func ParseClientSecret(raw string) (prefix string, err error) {
	return parsePrefixed(clientSecretPrefix, raw)
}

// ParseAuthorizationCode returns the lookup prefix from an `oac_…` string.
func ParseAuthorizationCode(raw string) (prefix string, err error) {
	return parsePrefixed(authzCodePrefix, raw)
}

// VerifyHash compares sha256(raw) to storedHash in constant time.
func VerifyHash(raw string, storedHash []byte) bool {
	if len(storedHash) != sha256.Size {
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return subtle.ConstantTimeCompare(sum[:], storedHash) == 1
}
