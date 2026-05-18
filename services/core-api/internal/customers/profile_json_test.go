package customers

import (
	"testing"
	"time"

	"sip-happens/pkg/domain"
)

func TestProfileViewFromDomain(t *testing.T) {
	c := &domain.Customer{
		ID:           "cust-1",
		Email:        "a@b.co",
		FullName:     "Ana",
		City:         "Bogotá",
		BirthDate:    time.Date(1990, 3, 15, 0, 0, 0, 0, time.UTC),
	}
	v := ProfileViewFromDomain(c)
	if v.ID != "cust-1" || v.Email != "a@b.co" || v.FullName != "Ana" || v.BirthDate != "1990-03-15" {
		t.Fatalf("view: %#v", v)
	}
}

func TestApplyProfilePatch_BirthDateAdult(t *testing.T) {
	c := &domain.Customer{}
	bd := "1995-06-01"
	if err := ApplyProfilePatch(c, ProfilePatch{BirthDate: &bd}, time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if c.BirthDate.IsZero() {
		t.Fatal("expected birth date set")
	}
}

func TestApplyProfilePatch_MinorRejected(t *testing.T) {
	c := &domain.Customer{}
	bd := "2010-01-01"
	if err := ApplyProfilePatch(c, ProfilePatch{BirthDate: &bd}, time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)); err != domain.ErrInvalidInput {
		t.Fatalf("got %v", err)
	}
}
