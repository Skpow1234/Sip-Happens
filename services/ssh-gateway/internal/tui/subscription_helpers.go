package tui

import (
	"fmt"
	"strings"

	"sip-happens/services/ssh-gateway/internal/api"
)

func formatSubscriptionInterval(interval string) string {
	switch strings.TrimSpace(interval) {
	case "week":
		return L.SubscriptionIntervalWeek
	case "month":
		return L.SubscriptionIntervalMonth
	default:
		return interval
	}
}

func formatSubscriptionLine(sub api.SubscriptionView) string {
	interval := formatSubscriptionInterval(sub.Interval)
	price := formatMoney(sub.Currency, sub.UnitAmountMinorUnits)
	return fmt.Sprintf("%-14s  %-10s  ×%d  %s/%s",
		sub.Status, sub.ProductID, sub.Qty, price, interval)
}

func subscriptionAddressLabel(addrs []api.AddressView, addressID string) string {
	addressID = strings.TrimSpace(addressID)
	if addressID == "" {
		return ""
	}
	for _, a := range addrs {
		if a.ID == addressID {
			return formatAddressLine(a)
		}
	}
	return addressID
}
