package customers

import (
	"time"

	"sip-happens/pkg/age"
	"sip-happens/pkg/domain"
)

// ProfileView is the public customer profile JSON (Terminal.shop-style snake_case).
type ProfileView struct {
	ID           string `json:"id"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	LegalID      string `json:"legal_id,omitempty"`
	FullName     string `json:"full_name,omitempty"`
	BirthDate    string `json:"birth_date,omitempty"`
	AddressLine1 string `json:"address_line1,omitempty"`
	Neighborhood string `json:"neighborhood,omitempty"`
	City         string `json:"city,omitempty"`
}

// ProfilePatch is a partial profile update (PATCH /customers/me, PUT /profile).
type ProfilePatch struct {
	City         *string `json:"city"`
	Neighborhood *string `json:"neighborhood"`
	AddressLine1 *string `json:"address_line1"`
	Phone        *string `json:"phone"`
	LegalID      *string `json:"legal_id"`
	FullName     *string `json:"full_name"`
	BirthDate    *string `json:"birth_date"`
	Email        *string `json:"email"`
}

func ProfileViewFromDomain(c *domain.Customer) ProfileView {
	out := ProfileView{
		ID:           string(c.ID),
		Email:        c.Email,
		Phone:        c.Phone,
		LegalID:      c.LegalID,
		FullName:     c.FullName,
		AddressLine1: c.AddressLine1,
		Neighborhood: c.Neighborhood,
		City:         c.City,
	}
	if !c.BirthDate.IsZero() {
		out.BirthDate = c.BirthDate.Format("2006-01-02")
	}
	return out
}

// ApplyProfilePatch merges patch into c. Returns domain.ErrInvalidInput for bad birth_date or under 18.
func ApplyProfilePatch(c *domain.Customer, patch ProfilePatch, now time.Time) error {
	if patch.City != nil {
		c.City = *patch.City
	}
	if patch.Neighborhood != nil {
		c.Neighborhood = *patch.Neighborhood
	}
	if patch.AddressLine1 != nil {
		c.AddressLine1 = *patch.AddressLine1
	}
	if patch.Phone != nil {
		c.Phone = *patch.Phone
	}
	if patch.LegalID != nil {
		c.LegalID = *patch.LegalID
	}
	if patch.FullName != nil {
		c.FullName = *patch.FullName
	}
	if patch.Email != nil {
		c.Email = *patch.Email
	}
	if patch.BirthDate != nil {
		bd, err := ParseBirthDate(*patch.BirthDate)
		if err != nil {
			return domain.ErrInvalidInput
		}
		if !age.IsAdult(bd, now) {
			return domain.ErrInvalidInput
		}
		c.BirthDate = bd
	}
	return nil
}
