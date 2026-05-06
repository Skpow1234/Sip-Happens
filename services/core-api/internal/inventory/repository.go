package inventory

import (
	"context"
	"database/sql"

	"sip-happens/services/core-api/internal/domain"
)

type Repository interface {
	// LockForUpdate must SELECT ... FOR UPDATE to protect reservation integrity.
	LockForUpdate(ctx context.Context, tx *sql.Tx, productID domain.ProductID) (*domain.Inventory, error)

	// Update writes updated inventory values.
	Update(ctx context.Context, tx *sql.Tx, inv domain.Inventory) error

	// Reservation ledger operations.
	CreateReservation(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID, qty int) error
	ListActiveReservations(ctx context.Context, tx *sql.Tx, orderID domain.OrderID) ([]Reservation, error)
	MarkReservationReleased(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID) error
	MarkReservationConsumed(ctx context.Context, tx *sql.Tx, orderID domain.OrderID, productID domain.ProductID) error
}

type Reservation struct {
	OrderID   domain.OrderID
	ProductID domain.ProductID
	Qty       int
	Status    string
}

