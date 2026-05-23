package orders

import (
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/catalog"
)

// OrderListView is the public order summary for GET /orders (no delivery PII).
type OrderListView struct {
	ID                 string     `json:"id"`
	DisplayRef         string     `json:"display_ref"`
	Status             string     `json:"status"`
	SubtotalMinorUnits int64      `json:"subtotal_minor_units"`
	ShippingMinorUnits int64      `json:"shipping_minor_units"`
	TotalMinorUnits    int64      `json:"total_minor_units"`
	Currency           string     `json:"currency"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// OrderDetailView is the public order shape for GET /orders/{id}.
type OrderDetailView struct {
	OrderListView
	City         string `json:"city,omitempty"`
	Neighborhood string `json:"neighborhood,omitempty"`
	AddressLine1 string `json:"address_line1,omitempty"`
	Phone        string `json:"phone,omitempty"`
	LegalID      string `json:"legal_id,omitempty"`
	FullName     string `json:"full_name,omitempty"`
}

// OrderItemView is a line item in order detail responses.
type OrderItemView struct {
	ProductID           string `json:"product_id"`
	SKU                 string `json:"sku"`
	Name                string `json:"name"`
	Qty                 int    `json:"qty"`
	UnitPriceMinorUnits int64  `json:"unit_price_minor_units"`
	Currency            string `json:"currency"`
	LineTotalMinorUnits int64  `json:"line_total_minor_units"`
}

// ShipmentView is shipment status on order detail.
type ShipmentView struct {
	Status       string `json:"status"`
	Carrier      string `json:"carrier,omitempty"`
	TrackingCode string `json:"tracking_code,omitempty"`
}

func OrderListViewFromDomain(o domain.Order) OrderListView {
	return OrderListView{
		ID:                 string(o.ID),
		DisplayRef:         o.DisplayRef,
		Status:             string(o.Status),
		SubtotalMinorUnits: o.SubtotalMinorUnits,
		ShippingMinorUnits: o.ShippingMinorUnits,
		TotalMinorUnits:    o.TotalMinorUnits,
		Currency:           o.Currency,
		ExpiresAt:          o.ExpiresAt,
		CreatedAt:          o.CreatedAt,
		UpdatedAt:          o.UpdatedAt,
	}
}

func OrderDetailViewFromDomain(o domain.Order) OrderDetailView {
	return OrderDetailView{
		OrderListView: OrderListViewFromDomain(o),
		City:          o.City,
		Neighborhood:  o.Neighborhood,
		AddressLine1:  o.AddressLine1,
		Phone:         o.Phone,
		LegalID:       o.LegalID,
		FullName:      o.FullName,
	}
}

func OrderItemViewFromDomain(it domain.OrderItem) OrderItemView {
	return OrderItemView{
		ProductID:           catalog.PublicID(it.ProductID),
		SKU:                 it.SKU,
		Name:                it.Name,
		Qty:                 it.Qty,
		UnitPriceMinorUnits: it.UnitPriceMinorUnits,
		Currency:            it.Currency,
		LineTotalMinorUnits: it.LineTotalMinorUnits,
	}
}
