package catalog

import (
	"context"
	"database/sql"

	"sip-happens/pkg/domain"
)

type Repository interface {
	GetProduct(ctx context.Context, q Querier, id domain.ProductID) (*domain.Product, error)
	GetProductBySKU(ctx context.Context, q Querier, sku string) (*domain.Product, error)
	ListActiveProducts(ctx context.Context, q Querier, filter ListCatalogFilter) (ListCatalogResult, error)
	GetCurrentPrice(ctx context.Context, q Querier, productID domain.ProductID, currency string) (*domain.Price, error)
}

// ListCatalogFilter selects a page of active catalog products.
type ListCatalogFilter struct {
	Currency string
	Limit    int
	Offset   int
	Query    string
	Style    string
	Brewery  string
	ABVMin   *float64
	ABVMax   *float64
	// Featured switches the result ordering from the default lexical sort
	// (by sku asc) to a "destacados" curation: most recently introduced
	// active products first. Used by the TUI's "destacados" tab and the
	// /view/init bootstrap so the first paint highlights fresh inventory.
	Featured bool
}

type ListCatalogResult struct {
	Items   []ProductWithPrice
	HasMore bool
}

type ProductWithPrice struct {
	Product domain.Product
	Price   domain.Price
	// AvailableQty is sellable units (on_hand - reserved - safety_stock), floored at 0.
	AvailableQty int `json:"available_qty"`
}

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
