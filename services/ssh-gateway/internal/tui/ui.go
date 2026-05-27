package tui

import (
	"strings"

	"sip-happens/services/ssh-gateway/internal/api"
)

func formatOrderLabel(id, displayRef string) string {
	if dr := strings.TrimSpace(displayRef); dr != "" {
		return dr
	}
	id = strings.TrimSpace(id)
	if len(id) > 12 {
		return id[:8] + "…"
	}
	return id
}

func formatShipmentSummary(orderStatus string, sh *api.ShipmentView) string {
	if sh != nil && strings.TrimSpace(sh.Status) != "" {
		st := strings.TrimSpace(sh.Status)
		var lines []string
		lines = append(lines, L.ShipmentLabel+" "+L.ShipmentStatusLabel(st))
		if sh.Carrier != "" {
			lines = append(lines, L.ShipmentCarrier+" "+sh.Carrier)
		}
		if sh.TrackingCode != "" {
			lines = append(lines, L.ShipmentTracking+" "+sh.TrackingCode)
		}
		switch st {
		case "NEW", "PENDING":
			lines = append(lines, L.ShipmentNew)
		case "PACKED":
			lines = append(lines, L.ShipmentPacked)
		case "HANDED_OVER":
			lines = append(lines, L.ShipmentHandedOver)
		case "IN_TRANSIT":
			lines = append(lines, L.ShipmentInTransit)
		case "DELIVERED":
			lines = append(lines, L.ShipmentDelivered)
		case "FAILED":
			lines = append(lines, L.ShipmentFailed)
		}
		if sh.TrackingCode != "" && (orderStatus == "SHIPPED" || st == "IN_TRANSIT" || st == "HANDED_OVER" || st == "DELIVERED") {
			lines = append(lines, L.ShipmentTrackHint)
		}
		return strings.Join(lines, "\n") + "\n"
	}
	switch orderStatus {
	case "PAID":
		return L.ShipmentPaid + "\n"
	case "FULFILLING":
		return L.ShipmentFulfilling + "\n"
	case "SHIPPED":
		return L.ShipmentShipped + "\n"
	default:
		return ""
	}
}
