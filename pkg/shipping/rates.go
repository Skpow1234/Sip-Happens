package shipping

import (
	"context"
	"database/sql"
	"strings"

	"github.com/lib/pq"
)

// DefaultItemWeightGrams is used for products that have no per-row
// weight_grams value yet — equivalent to a single 330 ml craft beer
// bottle with cardboard insert (~500 g). The same constant is used by
// the migration default; keep them in sync if you change it.
const DefaultItemWeightGrams = 500

// LookupAmountMinor returns the flat per-city shipping rate. Kept for
// callers that don't know cart weight (`GET /shipping/estimate` with no
// items, the email signup banner CTA, etc.). For checkout/cart use
// `LookupAmountMinorForWeight` so weight tiers are honored.
func LookupAmountMinor(ctx context.Context, db *sql.DB, cityKey, currency string) (int64, error) {
	if db == nil {
		return 0, nil
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "COP"
	}
	key := strings.TrimSpace(strings.ToLower(cityKey))
	if key == "" {
		return 0, nil
	}
	var n int64
	err := db.QueryRowContext(ctx, `
select amount_minor_units
from shipping_rates
where city_normalized = $1 and currency = $2
`, key, currency).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return n, nil
}

// LookupAmountMinorForWeight picks the cheapest tier whose
// `max_weight_grams` is >= the cart weight, or falls back to the flat
// `shipping_rates` row when no tier matches (weight > all tiers, or no
// tiers seeded for the city).
//
// Returns 0 when:
//   - db is nil (development / tests),
//   - cityKey is empty (caller hasn't picked a city yet),
//   - city has neither a tier row nor a flat row,
//
// in line with the existing `LookupAmountMinor` contract — the checkout
// engine treats a zero shipping cost as "free shipping" so a missing
// city doesn't block the API; the canonical-city validator upstream
// already rejects out-of-area requests.
func LookupAmountMinorForWeight(ctx context.Context, db *sql.DB, cityKey, currency string, totalWeightGrams int) (int64, error) {
	if db == nil {
		return 0, nil
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "COP"
	}
	key := strings.TrimSpace(strings.ToLower(cityKey))
	if key == "" {
		return 0, nil
	}
	if totalWeightGrams <= 0 {
		// Unknown weight — be conservative, fall back to the flat rate.
		return LookupAmountMinor(ctx, db, key, currency)
	}
	var n int64
	err := db.QueryRowContext(ctx, `
select amount_minor_units
from shipping_rate_tiers
where city_normalized = $1 and currency = $2 and max_weight_grams >= $3
order by max_weight_grams asc
limit 1
`, key, currency, totalWeightGrams).Scan(&n)
	if err == nil {
		return n, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	// Heavier than every tier → flat (or no tiers seeded at all → flat).
	return LookupAmountMinor(ctx, db, key, currency)
}

// ItemForWeight is the minimal shape needed to compute cart weight.
type ItemForWeight struct {
	ProductID string
	Qty       int
}

// TotalWeightGrams sums weight_grams * qty over the supplied items,
// using DefaultItemWeightGrams for any product whose row has a NULL
// weight. Products not found in the DB (deleted while cart was alive)
// also fall back to the default.
func TotalWeightGrams(ctx context.Context, db *sql.DB, items []ItemForWeight) (int, error) {
	if db == nil || len(items) == 0 {
		return 0, nil
	}
	productIDs := make([]string, 0, len(items))
	qtyByID := make(map[string]int, len(items))
	for _, it := range items {
		id := strings.TrimSpace(it.ProductID)
		if id == "" || it.Qty <= 0 {
			continue
		}
		if _, seen := qtyByID[id]; !seen {
			productIDs = append(productIDs, id)
		}
		qtyByID[id] += it.Qty
	}
	if len(productIDs) == 0 {
		return 0, nil
	}
	weights := make(map[string]int, len(productIDs))
	rows, err := db.QueryContext(ctx, `
select id, coalesce(weight_grams, $1)
from products where id = any($2)
`, DefaultItemWeightGrams, pq.Array(productIDs))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var w int
		if err := rows.Scan(&id, &w); err != nil {
			return 0, err
		}
		weights[id] = w
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	total := 0
	for id, qty := range qtyByID {
		w, ok := weights[id]
		if !ok {
			w = DefaultItemWeightGrams
		}
		total += w * qty
	}
	return total, nil
}

