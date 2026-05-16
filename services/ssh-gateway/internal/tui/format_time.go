package tui

import (
	"fmt"
	"time"
)

func formatExpiryCountdown(expiresAt *time.Time, now time.Time) string {
	if expiresAt == nil {
		return ""
	}
	left := expiresAt.Sub(now)
	if left <= 0 {
		return L.Expired
	}
	m := int(left.Minutes())
	s := int(left.Seconds()) % 60
	if m >= 60 {
		h := m / 60
		m = m % 60
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatExpiryAt(expiresAt *time.Time, now time.Time) string {
	if expiresAt == nil {
		return "—"
	}
	if expiresAt.Before(now) {
		return L.Expired
	}
	return expiresAt.In(now.Location()).Format("2006-01-02 15:04")
}
