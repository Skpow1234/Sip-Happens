package domain

import "time"

type ProductID string

type Product struct {
	ID        ProductID
	SKU       string
	Name      string
	Brewery   string
	Style     string
	ABV       float64
	VolumeML  int
	InvimaReg   string
	Description string
	IBU         *int
	// ImageURL is a public HTTPS URL for catalog/TUI display (object storage or CDN).
	ImageURL string
	IsActive  bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Price struct {
	ProductID        ProductID
	Currency         string
	AmountMinorUnits int64
	ValidFrom        time.Time
	ValidThrough     *time.Time
}

