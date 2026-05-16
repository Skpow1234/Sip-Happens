package admin

import (
	"strings"
	"testing"

	"sip-happens/pkg/redact"
)

func TestInsertAudit_RedactsStripeMaterialInPayload(t *testing.T) {
	payload := `{"note":"leaked sk_live_abc123xyz","client_secret":"cs_test_secret"}`
	sanitized := redact.JSON(payload)
	if strings.Contains(sanitized, "abc123xyz") || strings.Contains(sanitized, "cs_test_secret") {
		t.Fatalf("audit payload not redacted: %s", sanitized)
	}
}
