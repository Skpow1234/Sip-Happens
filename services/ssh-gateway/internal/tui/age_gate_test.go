package tui

import "testing"

func TestValidateAdultBirthDate(t *testing.T) {
	if err := validateAdultBirthDate("2010-01-01"); err == nil {
		t.Fatal("expected minor rejection")
	}
	if err := validateAdultBirthDate("1995-01-01"); err != nil {
		t.Fatal(err)
	}
}
