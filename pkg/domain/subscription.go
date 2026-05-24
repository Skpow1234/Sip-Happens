package domain

import "time"

// SubscriptionID identifies a row in customer_subscriptions.
type SubscriptionID string

// SubscriptionInterval is the billing cadence for a recurring subscription.
type SubscriptionInterval string

const (
	SubscriptionIntervalWeek  SubscriptionInterval = "week"
	SubscriptionIntervalMonth SubscriptionInterval = "month"
)

// SubscriptionStatus mirrors Stripe subscription lifecycle states used in-app.
type SubscriptionStatus string

const (
	SubscriptionStatusPending    SubscriptionStatus = "PENDING"
	SubscriptionStatusActive     SubscriptionStatus = "ACTIVE"
	SubscriptionStatusPastDue    SubscriptionStatus = "PAST_DUE"
	SubscriptionStatusPaused     SubscriptionStatus = "PAUSED"
	SubscriptionStatusCancelled  SubscriptionStatus = "CANCELLED"
	SubscriptionStatusIncomplete SubscriptionStatus = "INCOMPLETE"
)

// Subscription is a customer's recurring product enrollment.
type Subscription struct {
	ID                       SubscriptionID
	CustomerID               CustomerID
	ProductID                ProductID
	Qty                      int
	Currency                 string
	Interval                 SubscriptionInterval
	UnitAmountMinorUnits     int64
	Status                   SubscriptionStatus
	StripeSubscriptionID     string
	StripeSubscriptionItemID   string
	AddressID                AddressID
	CurrentPeriodStart       *time.Time
	CurrentPeriodEnd         *time.Time
	CancelledAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
