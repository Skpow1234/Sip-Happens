package customers

import (
	"context"
	"time"

	"sip-happens/pkg/domain"
)

// Service handles customer profile, cart, and consent persistence.
type Service interface {
	GetByID(ctx context.Context, id domain.CustomerID) (*domain.Customer, error)
	IDByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error)
	GetOrCreateByFingerprint(ctx context.Context, fp string) (domain.CustomerID, error)
	Update(ctx context.Context, c *domain.Customer) error
	SaveCheckoutProfile(ctx context.Context, id domain.CustomerID, in CheckoutProfile) error

	LoadCart(ctx context.Context, customerID domain.CustomerID) ([]CartLine, error)
	SaveCart(ctx context.Context, customerID domain.CustomerID, items []CartLine) error
	ClearCart(ctx context.Context, customerID domain.CustomerID) error
	LoadCartAddressID(ctx context.Context, customerID domain.CustomerID) (domain.AddressID, error)
	SetCartAddressID(ctx context.Context, customerID domain.CustomerID, addressID domain.AddressID) error
	LoadCartPaymentMethodID(ctx context.Context, customerID domain.CustomerID) (string, error)
	SetCartPaymentMethodID(ctx context.Context, customerID domain.CustomerID, paymentMethodID string) error
	RecordConsent(ctx context.Context, customerID domain.CustomerID, purpose string) error
	RecordAgeVerification(ctx context.Context, customerID domain.CustomerID, orderID domain.OrderID, birthDate, method string) error

	StripeCustomerID(ctx context.Context, id domain.CustomerID) (string, error)
	SetStripeCustomerID(ctx context.Context, id domain.CustomerID, stripeCustomerID string) error
}

// CheckoutProfile is delivery/contact data submitted at checkout.
type CheckoutProfile struct {
	City         string
	Neighborhood string
	AddressLine1 string
	Phone        string
	LegalID      string
	FullName     string
	BirthDate    string // YYYY-MM-DD; empty skips birth_date update
}

// CartLine is a persisted server-side cart row.
type CartLine struct {
	ProductID    string `json:"product_id"`
	SKU          string `json:"sku,omitempty"`
	Name         string `json:"name,omitempty"`
	Currency     string `json:"currency,omitempty"`
	UnitPrice    int64  `json:"unit_price_minor_units,omitempty"`
	Qty          int    `json:"qty"`
	AvailableQty int    `json:"available_qty,omitempty"`
}

// ParseBirthDate parses YYYY-MM-DD for profile updates.
func ParseBirthDate(s string) (time.Time, error) {
	return parseBirthDate(s)
}
