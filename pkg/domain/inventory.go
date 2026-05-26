package domain

import "time"

type Inventory struct {
	ProductID    ProductID
	OnHand       int
	Reserved     int
	SafetyStock  int
	LastModified time.Time
}

func (i Inventory) Available() int {
	return i.OnHand - i.Reserved - i.SafetyStock
}
