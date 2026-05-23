package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/orders"
)

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.Orders == nil || s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.Customers.IDByFingerprint(r.Context(), fp)
	if err != nil {
		if err == domain.ErrNotFound {
			w.Header().Set("content-type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"orders":   []orders.OrderListView{},
				"limit":    parseLimitQuery(r, 20, 100),
				"offset":   parseOffsetQuery(r),
				"has_more": false,
			})
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	limit := parseLimitQuery(r, 20, 100)
	offset := parseOffsetQuery(r)
	page, err := s.Orders.ListOrdersByCustomer(r.Context(), customerID, limit, offset)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	views := make([]orders.OrderListView, 0, len(page.Orders))
	for _, o := range page.Orders {
		views = append(views, orders.OrderListViewFromDomain(o))
	}
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"orders":   views,
		"limit":    limit,
		"offset":   offset,
		"has_more": page.HasMore,
	})
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ref := strings.TrimPrefix(r.URL.Path, "/orders/")
	if ref == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing id")
		return
	}
	if s.Orders == nil || s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.Customers.IDByFingerprint(r.Context(), fp)
	if err != nil {
		if err == domain.ErrNotFound {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "order not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	o, items, err := s.Orders.GetOrderForCustomer(r.Context(), customerID, ref)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if err == domain.ErrNotFound {
			status = http.StatusNotFound
			code = "not_found"
		}
		writeAPIError(w, r, status, code, "error")
		return
	}

	itemViews := make([]orders.OrderItemView, 0, len(items))
	for _, it := range items {
		itemViews = append(itemViews, orders.OrderItemViewFromDomain(it))
	}
	payload := map[string]any{
		"order": orders.OrderDetailViewFromDomain(*o),
		"items": itemViews,
	}
	if sh, err := s.shipmentViewForOrder(r.Context(), o.ID); err == nil && sh != nil {
		payload["shipment"] = sh
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) shipmentViewForOrder(ctx context.Context, orderID domain.OrderID) (*orders.ShipmentView, error) {
	raw, err := s.loadShipmentForOrder(ctx, orderID)
	if err != nil || raw == nil {
		return nil, err
	}
	out := &orders.ShipmentView{}
	if v, ok := raw["Status"].(string); ok {
		out.Status = v
	}
	if v, ok := raw["Carrier"].(string); ok && v != "" {
		out.Carrier = v
	}
	if v, ok := raw["TrackingCode"].(string); ok && v != "" {
		out.TrackingCode = v
	}
	if out.Status == "" {
		return nil, nil
	}
	return out, nil
}
