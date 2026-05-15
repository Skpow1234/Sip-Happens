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
	if id := strings.TrimSpace(m.state.lastOrderID); id != "" {
		if len(id) > 8 {
			id = id[:8]
		}
		parts = append(parts, "order "+id)
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

func formatShipmentSummary(orderStatus string, sh *api.ShipmentView) string {
	if sh != nil && strings.TrimSpace(sh.Status) != "" {
		var lines []string
		lines = append(lines, "Shipment: "+strings.TrimSpace(sh.Status))
		if sh.Carrier != "" {
			lines = append(lines, "Carrier: "+sh.Carrier)
		}
		if sh.TrackingCode != "" {
			lines = append(lines, "Tracking: "+sh.TrackingCode)
		}
		return strings.Join(lines, "\n") + "\n"
	}
	switch orderStatus {
	case "PAID":
		return "Fulfillment will begin shortly.\n"
	case "FULFILLING":
		return "Your order is being prepared for shipment.\n"
	case "SHIPPED":
		return "Your order has shipped.\n"
	default:
		return ""
	}
}
