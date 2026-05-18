package domain

import "time"

// AddressID identifies a row in customer_addresses.
type AddressID string

// CustomerAddress is a saved delivery address for a customer (pilot Colombia cities).
type CustomerAddress struct {
	ID             AddressID
	CustomerID     CustomerID
	Label          string
	FullName       string
	Phone          string
	LegalID        string
	AddressLine1   string
	Neighborhood   string
	City           string // Canonical display (Bogotá, Medellín, Cali)
	CityNormalized string // shipping_rates key (bogota, medellin, cali)
	IsDefault      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
