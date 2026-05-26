package subscriptions

import (
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/catalog"
)

// SubscriptionView is the public JSON shape for a subscription.
type SubscriptionView struct {
	ID                   string     `json:"id"`
	ProductID            string     `json:"product_id"`
	Qty                  int        `json:"qty"`
	Currency             string     `json:"currency"`
	Interval             string     `json:"interval"`
	UnitAmountMinorUnits int64      `json:"unit_amount_minor_units"`
	Status               string     `json:"status"`
	AddressID            string     `json:"address_id,omitempty"`
	CurrentPeriodStart   *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CancelledAt          *time.Time `json:"cancelled_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func SubscriptionViewFromDomain(sub domain.Subscription) SubscriptionView {
	v := SubscriptionView{
		ID:                   PublicID(sub.ID),
		ProductID:            catalog.PublicID(sub.ProductID),
		Qty:                  sub.Qty,
		Currency:             sub.Currency,
		Interval:             string(sub.Interval),
		UnitAmountMinorUnits: sub.UnitAmountMinorUnits,
		Status:               string(sub.Status),
		CurrentPeriodStart:   sub.CurrentPeriodStart,
		CurrentPeriodEnd:     sub.CurrentPeriodEnd,
		CancelledAt:          sub.CancelledAt,
		CreatedAt:            sub.CreatedAt,
		UpdatedAt:            sub.UpdatedAt,
	}
	if sub.AddressID != "" {
		v.AddressID = addresses.PublicID(sub.AddressID)
	}
	return v
}
