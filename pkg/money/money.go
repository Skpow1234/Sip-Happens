package money

import "fmt"

// Amount represents a monetary amount in the currency's smallest unit as defined by the PSP.
//
// For **COP on Stripe**, the amount is in **pesos** (zero-decimal), not centavos.
type Amount struct {
	Currency   string
	MinorUnits int64
}

func (a Amount) String() string {
	if a.Currency == "" {
		return fmt.Sprintf("%d", a.MinorUnits)
	}
	return fmt.Sprintf("%s %d", a.Currency, a.MinorUnits)
}

func COP(pesos int64) Amount {
	return Amount{Currency: "COP", MinorUnits: pesos}
}
