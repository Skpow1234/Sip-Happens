package http

import (
	"context"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/addresses"
)

// deliveryFieldOverrides supplies optional saved address id and per-field overrides.
type deliveryFieldOverrides struct {
	AddressID    string
	City         string
	Neighborhood string
	AddressLine1 string
	Phone        string
	LegalID      string
	FullName     string
}

func (s *Server) resolveOrderDelivery(ctx context.Context, customerID domain.CustomerID, o deliveryFieldOverrides) (deliverySnapshot, error) {
	profile := s.customerDeliveryProfile(ctx, customerID)

	var saved deliverySnapshot
	if ref := strings.TrimSpace(o.AddressID); ref != "" {
		if s.Addresses == nil || s.DB == nil {
			return deliverySnapshot{}, domain.ErrNotFound
		}
		a, err := addresses.ResolveAddress(ctx, s.Addresses, s.DB, customerID, ref)
		if err != nil {
			return deliverySnapshot{}, err
		}
		av := addresses.AddressViewFromDomain(*a)
		saved = deliverySnapshot{
			City:         av.City,
			Neighborhood: av.Neighborhood,
			AddressLine1: av.AddressLine1,
			Phone:        av.Phone,
			LegalID:      av.LegalID,
			FullName:     av.FullName,
		}
	}

	return deliverySnapshot{
		City:         pickDeliveryField(o.City, saved.City, profile.City),
		Neighborhood: pickDeliveryField(o.Neighborhood, saved.Neighborhood, profile.Neighborhood),
		AddressLine1: pickDeliveryField(o.AddressLine1, saved.AddressLine1, profile.AddressLine1),
		Phone:        pickDeliveryField(o.Phone, saved.Phone, profile.Phone),
		LegalID:      pickDeliveryField(o.LegalID, saved.LegalID, profile.LegalID),
		FullName:     pickDeliveryField(o.FullName, saved.FullName, profile.FullName),
	}, nil
}
