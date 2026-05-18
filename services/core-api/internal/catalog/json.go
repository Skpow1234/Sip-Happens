package catalog

import "sip-happens/pkg/domain"

// ProductView is the public catalog product shape (snake_case JSON).
type ProductView struct {
	ID          string  `json:"id"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Brewery     string  `json:"brewery"`
	Style       string  `json:"style"`
	ABV         float64 `json:"abv"`
	VolumeML    int     `json:"volume_ml"`
	InvimaReg   string  `json:"invima_reg"`
	Description string  `json:"description,omitempty"`
	IBU         *int    `json:"ibu,omitempty"`
	ImageURL    string  `json:"image_url,omitempty"`
}

// PriceView is the current price snapshot in catalog responses.
type PriceView struct {
	Currency         string `json:"currency"`
	AmountMinorUnits int64  `json:"amount_minor_units"`
}

// ItemView is one catalog row (list or nested in detail).
type ItemView struct {
	Product      ProductView `json:"product"`
	Price        PriceView   `json:"price"`
	AvailableQty int         `json:"available_qty"`
}

func ProductViewFromDomain(p domain.Product) ProductView {
	return ProductView{
		ID:          PublicID(p.ID),
		SKU:         p.SKU,
		Name:        p.Name,
		Brewery:     p.Brewery,
		Style:       p.Style,
		ABV:         p.ABV,
		VolumeML:    p.VolumeML,
		InvimaReg:   p.InvimaReg,
		Description: p.Description,
		IBU:         p.IBU,
		ImageURL:    p.ImageURL,
	}
}

func PriceViewFromDomain(pr domain.Price) PriceView {
	return PriceView{
		Currency:         pr.Currency,
		AmountMinorUnits: pr.AmountMinorUnits,
	}
}

func ItemViewFromProductWithPrice(pwp ProductWithPrice) ItemView {
	return ItemView{
		Product:      ProductViewFromDomain(pwp.Product),
		Price:        PriceViewFromDomain(pwp.Price),
		AvailableQty: pwp.AvailableQty,
	}
}
