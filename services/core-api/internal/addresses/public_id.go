package addresses

import (
	"strings"

	"github.com/google/uuid"

	"sip-happens/pkg/domain"
)

const publicIDPrefix = "addr_"

// PublicID returns the stable address id exposed in JSON.
func PublicID(id domain.AddressID) string {
	s := strings.TrimSpace(string(id))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, publicIDPrefix) {
		return s
	}
	if u, err := uuid.Parse(s); err == nil {
		return publicIDPrefix + strings.ReplaceAll(u.String(), "-", "")
	}
	return publicIDPrefix + strings.ReplaceAll(s, "-", "")
}

// AddressIDFromPublicID decodes addr_* to internal id when possible.
func AddressIDFromPublicID(pub string) (domain.AddressID, bool) {
	pub = strings.TrimSpace(pub)
	if !strings.HasPrefix(pub, publicIDPrefix) {
		return "", false
	}
	tail := strings.TrimPrefix(pub, publicIDPrefix)
	if len(tail) != 32 {
		return "", false
	}
	raw := tail[0:8] + "-" + tail[8:12] + "-" + tail[12:16] + "-" + tail[16:20] + "-" + tail[20:32]
	if _, err := uuid.Parse(raw); err != nil {
		return "", false
	}
	return domain.AddressID(raw), true
}
