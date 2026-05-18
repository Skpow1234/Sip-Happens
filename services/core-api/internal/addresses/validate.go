package addresses

import (
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/shipping"
)

// NewAddressInput is the data required to create a saved address.
type NewAddressInput struct {
	Label        string
	FullName     string
	Phone        string
	LegalID      string
	AddressLine1 string
	Neighborhood string
	City         string
	IsDefault    bool
}

// ValidateNewAddress checks required fields and normalizes city to pilot catalog values.
func ValidateNewAddress(in NewAddressInput) (canonicalCity string, cityKey string, err error) {
	if strings.TrimSpace(in.AddressLine1) == "" ||
		strings.TrimSpace(in.Neighborhood) == "" ||
		strings.TrimSpace(in.City) == "" {
		return "", "", domain.ErrInvalidInput
	}
	canon, err := shipping.CanonicalCity(in.City)
	if err != nil {
		return "", "", domain.ErrInvalidInput
	}
	key := shipping.CityKey(canon)
	if key == "" {
		return "", "", domain.ErrInvalidInput
	}
	return canon, key, nil
}
