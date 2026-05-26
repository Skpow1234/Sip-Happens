package tokens

import (
	"strings"

	"github.com/google/uuid"

	"sip-happens/pkg/domain"
)

// PublicIDPrefix is the stable customer-facing prefix for personal access token IDs.
const PublicIDPrefix = "pat_"

// PublicID returns the stable token id exposed in JSON (pat_<32 hex>).
//
// The raw token returned at creation uses the same prefix but has a SECOND
// underscore section holding the high-entropy secret (see GenerateRawToken).
// Public IDs never round-trip the secret, only the row UUID.
func PublicID(id domain.PersonalAccessTokenID) string {
	s := strings.TrimSpace(string(id))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, PublicIDPrefix) {
		return s
	}
	if u, err := uuid.Parse(s); err == nil {
		return PublicIDPrefix + strings.ReplaceAll(u.String(), "-", "")
	}
	return PublicIDPrefix + strings.ReplaceAll(s, "-", "")
}

// IDFromPublicID decodes pat_<32 hex> back to the internal UUID id.
// Returns false when pub is not a token public id (e.g. it's the raw token,
// which has an extra `_<secret>` section).
func IDFromPublicID(pub string) (domain.PersonalAccessTokenID, bool) {
	pub = strings.TrimSpace(pub)
	if !strings.HasPrefix(pub, PublicIDPrefix) {
		return "", false
	}
	tail := strings.TrimPrefix(pub, PublicIDPrefix)
	if strings.Contains(tail, "_") {
		// Looks like a raw token (pat_<prefix>_<secret>), not a bare id.
		return "", false
	}
	if len(tail) != 32 {
		return "", false
	}
	raw := tail[0:8] + "-" + tail[8:12] + "-" + tail[12:16] + "-" + tail[16:20] + "-" + tail[20:32]
	if _, err := uuid.Parse(raw); err != nil {
		return "", false
	}
	return domain.PersonalAccessTokenID(raw), true
}
