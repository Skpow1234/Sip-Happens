package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// Raw token format: pat_<32-hex-prefix>_<43-char-base64url-secret>
//
// The prefix is the cheap lookup key (indexed in personal_access_tokens.token_prefix)
// and is stored in clear text. The full raw string is SHA-256 hashed and the 32-byte
// digest is stored in token_hash for constant-time verification.

const (
	// RawTokenPrefix is the visible label users will recognise on a raw token string.
	RawTokenPrefix = "pat_"
	// prefixBytes controls the byte length of the random lookup prefix (32 hex chars).
	prefixBytes = 16
	// secretBytes controls the byte length of the high-entropy secret portion.
	// 32 bytes → 43 base64url chars (no padding) → ~256 bits of entropy.
	secretBytes = 32
)

// GeneratedToken bundles the user-visible secret with the values persisted in DB.
type GeneratedToken struct {
	// Raw is the only string the user ever sees. Format: pat_<prefix>_<secret>.
	Raw string
	// Prefix is the indexed lookup column; equals the middle hex section of Raw.
	Prefix string
	// Hash is the SHA-256 of the full Raw string; persisted in token_hash.
	Hash []byte
}

// GenerateRawToken builds a fresh PAT secret using crypto/rand.
//
// The returned GeneratedToken.Raw must be shown to the user exactly once; the
// caller persists Prefix and Hash and discards Raw.
func GenerateRawToken() (GeneratedToken, error) {
	prefixBuf := make([]byte, prefixBytes)
	if _, err := rand.Read(prefixBuf); err != nil {
		return GeneratedToken{}, err
	}
	secretBuf := make([]byte, secretBytes)
	if _, err := rand.Read(secretBuf); err != nil {
		return GeneratedToken{}, err
	}
	prefix := hex.EncodeToString(prefixBuf)
	secret := base64.RawURLEncoding.EncodeToString(secretBuf)
	raw := RawTokenPrefix + prefix + "_" + secret
	sum := sha256.Sum256([]byte(raw))
	return GeneratedToken{
		Raw:    raw,
		Prefix: prefix,
		Hash:   sum[:],
	}, nil
}

// ErrInvalidRawToken indicates the provided string is not a well-formed PAT.
var ErrInvalidRawToken = errors.New("tokens: invalid raw token format")

// ParseRawToken extracts the lookup prefix from a raw token string.
//
// It validates the visible structure only; the caller must still load the row
// by prefix and compare the SHA-256 of the full raw string to the stored hash
// with VerifyRawToken before trusting the result.
func ParseRawToken(raw string) (prefix string, err error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, RawTokenPrefix) {
		return "", ErrInvalidRawToken
	}
	body := strings.TrimPrefix(raw, RawTokenPrefix)
	parts := strings.SplitN(body, "_", 2)
	if len(parts) != 2 {
		return "", ErrInvalidRawToken
	}
	prefix = parts[0]
	if len(prefix) != prefixBytes*2 {
		return "", ErrInvalidRawToken
	}
	if _, err := hex.DecodeString(prefix); err != nil {
		return "", ErrInvalidRawToken
	}
	if len(parts[1]) == 0 {
		return "", ErrInvalidRawToken
	}
	return prefix, nil
}

// VerifyRawToken returns true when sha256(raw) matches the stored hash in
// constant time.
func VerifyRawToken(raw string, storedHash []byte) bool {
	if len(storedHash) != sha256.Size {
		return false
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return subtle.ConstantTimeCompare(sum[:], storedHash) == 1
}
