package shipping

import (
	"context"
	"database/sql"
	"strings"
)

// LookupAmountMinor returns shipping in minor units for shipping_rates.city_normalized (e.g. bogota).
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
