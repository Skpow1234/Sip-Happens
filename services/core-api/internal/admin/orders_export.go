package admin

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"sip-happens/pkg/domain"
)

// WriteOrdersCSV writes finance-friendly order rows (one line per order).
func WriteOrdersCSV(w io.Writer, orders []domain.Order) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"order_id",
		"display_ref",
		"status",
		"created_at",
		"updated_at",
		"customer_id",
		"full_name",
		"legal_id",
		"phone",
		"city",
		"neighborhood",
		"address_line1",
		"currency",
		"subtotal_minor_units",
		"shipping_minor_units",
		"total_minor_units",
		"stripe_checkout_session_id",
		"stripe_payment_intent_id",
		"expires_at",
	}); err != nil {
		return err
	}
	for _, o := range orders {
		if err := cw.Write(orderCSVRow(o)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func orderCSVRow(o domain.Order) []string {
	return []string{
		string(o.ID),
		o.DisplayRef,
		string(o.Status),
		formatCSVTime(o.CreatedAt),
		formatCSVTime(o.UpdatedAt),
		string(o.CustomerID),
		o.FullName,
		o.LegalID,
		o.Phone,
		o.City,
		o.Neighborhood,
		o.AddressLine1,
		o.Currency,
		fmt.Sprintf("%d", o.SubtotalMinorUnits),
		fmt.Sprintf("%d", o.ShippingMinorUnits),
		fmt.Sprintf("%d", o.TotalMinorUnits),
		o.StripeCheckoutSessionID,
		o.StripePaymentIntentID,
		formatCSVTimePtr(o.ExpiresAt),
	}
}

func formatCSVTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatCSVTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
