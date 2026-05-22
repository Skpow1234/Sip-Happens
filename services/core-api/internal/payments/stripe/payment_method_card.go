package stripe

import (
	"strings"

	"github.com/stripe/stripe-go/v82"
)

// PaymentMethodCard is safe card metadata for API responses (no PAN).
type PaymentMethodCard struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Brand     string `json:"brand,omitempty"`
	Last4     string `json:"last4,omitempty"`
	ExpMonth  int64  `json:"exp_month,omitempty"`
	ExpYear   int64  `json:"exp_year,omitempty"`
	IsDefault bool   `json:"is_default"`
}

func paymentMethodCardFromStripe(pm *stripe.PaymentMethod, defaultID string) *PaymentMethodCard {
	if pm == nil {
		return nil
	}
	out := &PaymentMethodCard{
		ID:        pm.ID,
		Type:      string(pm.Type),
		IsDefault: defaultID != "" && pm.ID == defaultID,
	}
	if pm.Card != nil {
		out.Brand = string(pm.Card.Brand)
		out.Last4 = pm.Card.Last4
		out.ExpMonth = pm.Card.ExpMonth
		out.ExpYear = pm.Card.ExpYear
	}
	return out
}

func defaultPaymentMethodID(cust *stripe.Customer) string {
	if cust == nil || cust.InvoiceSettings == nil || cust.InvoiceSettings.DefaultPaymentMethod == nil {
		return ""
	}
	return strings.TrimSpace(cust.InvoiceSettings.DefaultPaymentMethod.ID)
}
