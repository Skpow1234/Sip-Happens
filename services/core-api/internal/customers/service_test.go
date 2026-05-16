package customers

import (
	"testing"
)

func TestParseBirthDate(t *testing.T) {
	if _, err := ParseBirthDate("1995-01-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBirthDate("not-a-date"); err == nil {
		t.Fatal("expected error")
	}
}
