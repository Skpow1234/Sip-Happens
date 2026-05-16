package tui

import (
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/age"
)

const DefaultPrivacyPolicyURL = "https://sip-happens.local/privacy"

// DefaultConsumerRightsURL is the SIC consumer-protection reference (DESIGN §5.3).
const DefaultConsumerRightsURL = "https://www.sic.gov.co/proteccion-del-consumidor"

// DefaultDataProcessingPolicyURL is linked at checkout consent (Ley 1581).
const DefaultDataProcessingPolicyURL = "https://sip-happens.local/legal/data-processing"

// birthDateDigits strips separators; only 0-9 are kept.
func birthDateDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatBirthDateDigits formats up to 8 digits as AAAA-MM-DD with auto hyphens while typing.
func formatBirthDateDigits(digits string) string {
	if len(digits) > 8 {
		digits = digits[:8]
	}
	n := len(digits)
	if n == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(digits[:minInt(n, 4)])
	if n >= 4 {
		out.WriteByte('-')
	}
	if n > 4 {
		out.WriteString(digits[4:minInt(n, 6)])
	}
	if n >= 6 {
		out.WriteByte('-')
	}
	if n > 6 {
		out.WriteString(digits[6:n])
	}
	return out.String()
}

func appendBirthDateInput(current string, r rune) string {
	if r < '0' || r > '9' {
		return current
	}
	d := birthDateDigits(current)
	if len(d) >= 8 {
		return formatBirthDateDigits(d)
	}
	return formatBirthDateDigits(d + string(r))
}

func backspaceBirthDateInput(current string) string {
	d := birthDateDigits(current)
	if len(d) == 0 {
		return ""
	}
	return formatBirthDateDigits(d[:len(d)-1])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func parseBirthDateInput(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("%s", L.ErrBirthRequired)
	}
	// Allow partial display values by normalizing to digit-only parse input.
	if len(birthDateDigits(s)) == 8 {
		d := birthDateDigits(s)
		s = d[:4] + "-" + d[4:6] + "-" + d[6:8]
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s", L.ErrBirthFormat)
	}
	return t, nil
}

func validateAdultBirthDate(s string) error {
	bd, err := parseBirthDateInput(s)
	if err != nil {
		return err
	}
	if !age.IsAdult(bd, time.Now()) {
		return fmt.Errorf("%s", L.ErrBirthMinor)
	}
	return nil
}
