package subscriptions

import (
	"strings"

	"github.com/google/uuid"

	"sip-happens/pkg/domain"
)

const publicIDPrefix = "sub_"

// PublicID returns the stable subscription id exposed in JSON.
func PublicID(id domain.SubscriptionID) string {
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

// SubscriptionIDFromPublicID decodes sub_* to internal id when possible.
func SubscriptionIDFromPublicID(pub string) (domain.SubscriptionID, bool) {
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
	return domain.SubscriptionID(raw), true
}
