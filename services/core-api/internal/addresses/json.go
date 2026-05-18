package addresses

import "sip-happens/pkg/domain"

// AddressView is the public JSON shape for saved addresses.
type AddressView struct {
	ID           string `json:"id"`
	Label        string `json:"label,omitempty"`
	FullName     string `json:"full_name,omitempty"`
	Phone        string `json:"phone,omitempty"`
	LegalID      string `json:"legal_id,omitempty"`
	AddressLine1 string `json:"address_line1"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	IsDefault    bool   `json:"is_default"`
}

func AddressViewFromDomain(a domain.CustomerAddress) AddressView {
	return AddressView{
		ID:           PublicID(a.ID),
		Label:        a.Label,
		FullName:     a.FullName,
		Phone:        a.Phone,
		LegalID:      a.LegalID,
		AddressLine1: a.AddressLine1,
		Neighborhood: a.Neighborhood,
		City:         a.City,
		IsDefault:    a.IsDefault,
	}
}
