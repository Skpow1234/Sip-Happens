package admin

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestValidateProductPublish(t *testing.T) {
	if err := validateProductPublish("INVIMA-1", true); err != nil {
		t.Fatal(err)
	}
	if err := validateProductPublish("", false); err != nil {
		t.Fatal(err)
	}
	if err := validateProductPublish("", true); err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid_input, got %v", err)
	}
	if err := validateProductPublish("  ", true); err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid_input for whitespace, got %v", err)
	}
}
