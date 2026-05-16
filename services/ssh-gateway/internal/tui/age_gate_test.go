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

func TestFormatBirthDateDigits_AutoHyphens(t *testing.T) {
	tests := []struct {
		digits string
		want   string
	}{
		{"", ""},
		{"2", "2"},
		{"2000", "2000-"},
		{"20005", "2000-5"},
		{"200005", "2000-05-"},
		{"20000516", "2000-05-16"},
	}
	for _, tc := range tests {
		got := formatBirthDateDigits(tc.digits)
		if got != tc.want {
			t.Fatalf("digits %q: got %q want %q", tc.digits, got, tc.want)
		}
	}
}

func TestAppendBirthDateInput(t *testing.T) {
	var s string
	for _, r := range "20000516" {
		s = appendBirthDateInput(s, r)
	}
	if s != "2000-05-16" {
		t.Fatalf("got %q", s)
	}
}
