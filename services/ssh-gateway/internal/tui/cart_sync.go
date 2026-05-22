package tui

import "sip-happens/services/ssh-gateway/internal/api"

func cartLinesToAPI(lines []CartLine) []api.CartLine {
	out := make([]api.CartLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, api.CartLine{
			ProductID:    l.ProductID,
			SKU:          l.SKU,
			Name:         l.Name,
			Currency:     l.Currency,
			UnitPrice:    l.UnitPrice,
			Qty:          l.Qty,
			AvailableQty: l.AvailableQty,
		})
	}
	return out
}

func cartLinesFromAPI(view *api.CartView) []CartLine {
	lines := api.CartLinesFromView(view)
	out := make([]CartLine, 0, len(lines))
	for _, it := range lines {
		out = append(out, CartLine{
			ProductID:    it.ProductID,
			SKU:          it.SKU,
			Name:         it.Name,
			Currency:     it.Currency,
			UnitPrice:    it.UnitPrice,
			Qty:          it.Qty,
			AvailableQty: it.AvailableQty,
		})
	}
	return out
}
