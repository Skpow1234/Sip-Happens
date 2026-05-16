package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"sip-happens/services/ssh-gateway/internal/api"
)

func (m Model) statusBar() string {
	var parts []string
	if m.busy {
		parts = append(parts, "loading…")
	}
	if n := m.state.cartCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("cart %d", n))
	}
	if label := m.state.orderLabel(); label != "" {
		parts = append(parts, "order "+label)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "  |  ")
}

func (m Model) withFooter(body string) string {
	bar := m.statusBar()
	if bar == "" {
		return body
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	return body + "\n\n" + style.Render(bar)
}

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
		lines = append(lines, "Shipment: "+st)
		if sh.Carrier != "" {
			lines = append(lines, "Carrier: "+sh.Carrier)
		}
		if sh.TrackingCode != "" {
			lines = append(lines, "Tracking: "+sh.TrackingCode)
		}
		if sh.TrackingCode != "" && (orderStatus == "SHIPPED" || st == "IN_TRANSIT" || st == "HANDED_OVER" || st == "DELIVERED") {
			lines = append(lines, "Track your package with the code above.")
		}
		return strings.Join(lines, "\n") + "\n"
	}
	switch orderStatus {
	case "PAID":
		return "Fulfillment will begin shortly.\n"
	case "FULFILLING":
		return "Your order is being prepared for shipment.\n"
	case "SHIPPED":
		return "Your order has shipped. Tracking will appear when the carrier scan is available.\n"
	default:
		return ""
	}
}
