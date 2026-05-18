package addresses

import (
	"testing"

	"sip-happens/pkg/domain"
)

func TestValidateNewAddress_PilotCity(t *testing.T) {
	city, key, err := ValidateNewAddress(NewAddressInput{
		AddressLine1: "Calle 1",
		Neighborhood: "Centro",
		City:         "bogota",
	})
	if err != nil {
		t.Fatal(err)
	}
	if city != "Bogotá" || key != "bogota" {
		t.Fatalf("city=%q key=%q", city, key)
	}
}

func TestValidateNewAddress_RejectUnknownCity(t *testing.T) {
	_, _, err := ValidateNewAddress(NewAddressInput{
		AddressLine1: "Calle 1",
		Neighborhood: "Centro",
		City:         "Cartagena",
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("got %v", err)
	}
}
