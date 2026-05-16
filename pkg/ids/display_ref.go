package ids

import (
	"crypto/rand"
	"fmt"
)

// crockfordBase32 excludes ambiguous characters (I,L,O,U).
const crockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewOrderDisplayRef returns a short human-friendly order reference (e.g. SH-A1B2C3D4).
func NewOrderDisplayRef() (string, error) {
	var body [8]byte
	for i := range body {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		body[i] = crockfordBase32[int(b[0])%32]
	}
	return fmt.Sprintf("SH-%s", string(body[:])), nil
}
