package oauthapps

import (
	"strings"

	"github.com/google/uuid"

	"sip-happens/pkg/domain"
)

// PublicIDPrefix is the stable customer-facing prefix for OAuth client row IDs.
const PublicIDPrefix = "oc_"

// PublicID converts the internal UUID row id to its `oc_<32 hex>` form.
//
// Distinct from the public OAuth `client_id` string (which is the value the
// third-party app embeds in its config) — see GenerateClientID.
func PublicID(id domain.OAuthClientID) string {
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

// IDFromPublicID decodes `oc_<32 hex>` back to the internal UUID id.
func IDFromPublicID(pub string) (domain.OAuthClientID, bool) {
	pub = strings.TrimSpace(pub)
	if !strings.HasPrefix(pub, PublicIDPrefix) {
		return "", false
	}
	tail := strings.TrimPrefix(pub, PublicIDPrefix)
	if strings.Contains(tail, "_") {
		return "", false
	}
	if len(tail) != 32 {
		return "", false
	}
	raw := tail[0:8] + "-" + tail[8:12] + "-" + tail[12:16] + "-" + tail[16:20] + "-" + tail[20:32]
	if _, err := uuid.Parse(raw); err != nil {
		return "", false
	}
	return domain.OAuthClientID(raw), true
}
