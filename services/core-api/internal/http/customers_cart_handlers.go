package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/cart"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
)

func (s *Server) handleCart(w http.ResponseWriter, r *http.Request) {
	if s.Customers == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	fp := r.Header.Get("X-SSH-Fingerprint")
	if fp == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing fingerprint")
		return
	}
	customerID, err := s.Customers.GetOrCreateByFingerprint(r.Context(), fp)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleCartGet(w, r, customerID)
	case http.MethodPut:
		s.handleCartPut(w, r, customerID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCartGet(w http.ResponseWriter, r *http.Request, customerID domain.CustomerID) {
	stored, err := s.Customers.LoadCart(r.Context(), customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	view, err := s.buildCartView(r, stored)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.writeCartJSON(w, view)
}

func (s *Server) handleCartPut(w http.ResponseWriter, r *http.Request, customerID domain.CustomerID) {
	var body struct {
		Items []customers.CartLine `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if body.Items == nil {
		body.Items = []customers.CartLine{}
	}
	if s.Catalog == nil || s.DB == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
		return
	}
	currency := cartCurrencyFromRequest(r)
	normalized := make([]customers.CartLine, 0, len(body.Items))
	for _, it := range body.Items {
		if it.Qty <= 0 {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid cart line")
			return
		}
		ref := strings.TrimSpace(it.ProductID)
		if ref == "" {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid cart line")
			return
		}
		p, err := catalog.ResolveProduct(r.Context(), s.Catalog, s.DB, ref)
		if err != nil {
			if err == domain.ErrNotFound {
				writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "unknown product")
				return
			}
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		price, err := s.Catalog.GetCurrentPrice(r.Context(), s.DB, p.ID, currency)
		if err != nil {
			if err == domain.ErrNotFound {
				writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "no price for currency")
				return
			}
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		normalized = append(normalized, customers.CartLine{
			ProductID:    catalog.PublicID(p.ID),
			SKU:          p.SKU,
			Name:         p.Name,
			Currency:     price.Currency,
			UnitPrice:    price.AmountMinorUnits,
			Qty:          it.Qty,
			AvailableQty: s.availableQty(r.Context(), p.ID),
		})
	}

	if err := s.Customers.SaveCart(r.Context(), customerID, normalized); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	view, err := s.buildCartView(r, normalized)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.writeCartJSON(w, view)
}

func (s *Server) buildCartView(r *http.Request, stored []customers.CartLine) (cart.View, error) {
	if s.Catalog == nil || s.DB == nil {
		return cart.View{Currency: cartCurrencyFromRequest(r), Items: []cart.ItemView{}}, nil
	}
	avail := func(ctx context.Context, productID domain.ProductID) int {
		return s.availableQty(ctx, productID)
	}
	return cart.BuildView(r.Context(), s.Catalog, s.DB, avail, cartCurrencyFromRequest(r), stored)
}

func cartCurrencyFromRequest(r *http.Request) string {
	c := strings.TrimSpace(r.URL.Query().Get("currency"))
	if c == "" {
		return "COP"
	}
	return c
}

func (s *Server) writeCartJSON(w http.ResponseWriter, view cart.View) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"cart": view})
}
