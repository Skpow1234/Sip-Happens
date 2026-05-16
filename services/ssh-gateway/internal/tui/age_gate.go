package tui

import (
	"fmt"
	"strings"
	"time"

	"sip-happens/pkg/age"
)

const DefaultPrivacyPolicyURL = "https://sip-happens.local/privacy"

func parseBirthDateInput(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("enter your birth date (YYYY-MM-DD)")
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("use format YYYY-MM-DD (e.g. 1995-06-15)")
	}
	return t, nil
}

func validateAdultBirthDate(s string) error {
	bd, err := parseBirthDateInput(s)
	if err != nil {
		return err
	}
	if !age.IsAdult(bd, time.Now()) {
		return fmt.Errorf("you must be 18 or older to use Sip-Happens")
	}
	return nil
}
