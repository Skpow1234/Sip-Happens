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

func parseBirthDateInput(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("%s", L.ErrBirthRequired)
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
