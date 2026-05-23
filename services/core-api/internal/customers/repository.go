package customers

import (
	"context"
	"database/sql"
	"time"

	"sip-happens/pkg/domain"
)

type Repository interface {
	GetByID(ctx context.Context, q Querier, id domain.CustomerID) (*domain.Customer, error)
	IDByFingerprint(ctx context.Context, q Querier, fp string) (domain.CustomerID, error)
	InsertByFingerprint(ctx context.Context, q Querier, id domain.CustomerID, fp string) error
	Update(ctx context.Context, q Querier, c *domain.Customer) error
	SaveCheckoutProfile(ctx context.Context, q Querier, id domain.CustomerID, city, neighborhood, addr, phone, legalID, fullName string, birth any) error

	LoadCart(ctx context.Context, q Querier, customerID domain.CustomerID) ([]CartLine, error)
	SaveCart(ctx context.Context, q Querier, customerID domain.CustomerID, itemsJSON []byte) error
	ClearCart(ctx context.Context, q Querier, customerID domain.CustomerID) error
	LoadCartAddressID(ctx context.Context, q Querier, customerID domain.CustomerID) (domain.AddressID, error)
	SetCartAddressID(ctx context.Context, q Querier, customerID domain.CustomerID, addressID domain.AddressID) error
	LoadCartPaymentMethodID(ctx context.Context, q Querier, customerID domain.CustomerID) (string, error)
	SetCartPaymentMethodID(ctx context.Context, q Querier, customerID domain.CustomerID, paymentMethodID string) error
	RecordConsent(ctx context.Context, q Querier, customerID domain.CustomerID, purpose string) error
	InsertAgeVerification(ctx context.Context, q Querier, id string, customerID domain.CustomerID, orderID domain.OrderID, birthDate time.Time, affirmedAt time.Time, method string) error

	StripeCustomerID(ctx context.Context, q Querier, id domain.CustomerID) (string, error)
	SetStripeCustomerID(ctx context.Context, q Querier, id domain.CustomerID, stripeCustomerID string) error
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
