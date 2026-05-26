package cart

import (
	"context"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
)

// ItemView is one cart line with current catalog product/price snapshot.
type ItemView struct {
	Qty                 int                 `json:"qty"`
	Product             catalog.ProductView `json:"product"`
	Price               catalog.PriceView   `json:"price"`
	LineTotalMinorUnits int64               `json:"line_total_minor_units"`
	AvailableQty        int                 `json:"available_qty"`
}

// View is the public GET/PUT /cart payload (Terminal.shop-style cart summary).
//
// ShippingMinorUnits and TotalMinorUnits are populated by the HTTP layer only
// when an address has been attached to the cart AND its city resolves to a
// known shipping rate — without an address we can't price shipping, so both
// fields stay at zero (and TotalMinorUnits matches SubtotalMinorUnits). This
// lets clients preview the full grand total before /cart/convert without
// running a parallel /shipping/estimate call.
type View struct {
	Currency           string                            `json:"currency"`
	Items              []ItemView                        `json:"items"`
	SubtotalMinorUnits int64                             `json:"subtotal_minor_units"`
	ShippingMinorUnits int64                             `json:"shipping_minor_units"`
	TotalMinorUnits    int64                             `json:"total_minor_units"`
	ItemCount          int                               `json:"item_count"`
	AddressID          string                            `json:"address_id,omitempty"`
	Address            *addresses.AddressView            `json:"address,omitempty"`
	PaymentMethodID    string                            `json:"payment_method_id,omitempty"`
	PaymentMethod      *stripepayments.PaymentMethodCard `json:"payment_method,omitempty"`
}

// AvailableQtyFunc returns sellable quantity for a product.
type AvailableQtyFunc func(ctx context.Context, productID domain.ProductID) int

// BuildView resolves stored cart lines against catalog and computes subtotal in currency.
func BuildView(
	ctx context.Context,
	cat catalog.Repository,
	q catalog.Querier,
	availableQty AvailableQtyFunc,
	currency string,
	stored []customers.CartLine,
) (View, error) {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "COP"
	}
	if stored == nil {
		stored = []customers.CartLine{}
	}
	out := View{
		Currency: currency,
		Items:    []ItemView{},
	}
	if cat == nil || q == nil {
		return out, nil
	}
	for _, line := range stored {
		if line.Qty <= 0 {
			continue
		}
		ref := strings.TrimSpace(line.ProductID)
		if ref == "" {
			continue
		}
		p, err := catalog.ResolveProduct(ctx, cat, q, ref)
		if err != nil {
			if err == domain.ErrNotFound {
				continue
			}
			return View{}, err
		}
		price, err := cat.GetCurrentPrice(ctx, q, p.ID, currency)
		if err != nil {
			if err == domain.ErrNotFound {
				continue
			}
			return View{}, err
		}
		avail := 0
		if availableQty != nil {
			avail = availableQty(ctx, p.ID)
		}
		unit := price.AmountMinorUnits
		lineTotal := unit * int64(line.Qty)
		out.Items = append(out.Items, ItemView{
			Qty:                 line.Qty,
			Product:             catalog.ProductViewFromDomain(*p),
			Price:               catalog.PriceViewFromDomain(*price),
			LineTotalMinorUnits: lineTotal,
			AvailableQty:        avail,
		})
		out.SubtotalMinorUnits += lineTotal
		out.ItemCount += line.Qty
	}
	// Default Total == Subtotal until the HTTP layer attaches an address and
	// computes shipping. Doing it here keeps the field non-zero for empty
	// carts (subtotal=0, total=0) and avoids a separate "did we compute totals
	// yet" sentinel.
	out.TotalMinorUnits = out.SubtotalMinorUnits
	return out, nil
}
