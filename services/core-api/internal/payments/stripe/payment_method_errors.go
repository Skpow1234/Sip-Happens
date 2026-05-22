package stripe

import "errors"

var (
	ErrPaymentMethodNotFound = errors.New("payment method not found")
	ErrPaymentMethodNotOwned = errors.New("payment method not owned by customer")
)
