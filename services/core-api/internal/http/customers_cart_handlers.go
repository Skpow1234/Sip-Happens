package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/cart"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
)

var errCartLineInvalid = errors.New("invalid cart line")

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

func (s *Server) handleCartItemByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
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

	productRef := strings.TrimPrefix(r.URL.Path, "/cart/items/")
	if productRef == "" {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "missing product id")
		return
	}

	var body struct {
		Qty int `json:"qty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, r, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if body.Qty < 0 {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid qty")
		return
	}

	stored, err := s.Customers.LoadCart(r.Context(), customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	if body.Qty == 0 {
		if s.Catalog == nil || s.DB == nil {
			writeAPIError(w, r, http.StatusInternalServerError, "misconfigured", "server misconfigured")
			return
		}
		p, err := catalog.ResolveProduct(r.Context(), s.Catalog, s.DB, productRef)
		if err != nil {
			if err == domain.ErrNotFound {
				writeAPIError(w, r, http.StatusNotFound, "not_found", "unknown product")
				return
			}
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		stored = cart.ApplyItemQty(stored, p, customers.CartLine{}, 0)
	} else {
		line, err := s.normalizeCartLine(r, productRef, body.Qty)
		if err != nil {
			s.writeCartLineError(w, r, err)
			return
		}
		p, err := catalog.ResolveProduct(r.Context(), s.Catalog, s.DB, productRef)
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		stored = cart.ApplyItemQty(stored, p, line, body.Qty)
	}

	_ = s.saveCartAndWrite(w, r, customerID, stored)
}

func (s *Server) handleCartGet(w http.ResponseWriter, r *http.Request, customerID domain.CustomerID) {
	stored, err := s.Customers.LoadCart(r.Context(), customerID)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	view, err := s.buildCartView(r, customerID, stored)
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
	normalized := make([]customers.CartLine, 0, len(body.Items))
	for _, it := range body.Items {
		if it.Qty <= 0 {
			writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid cart line")
			return
		}
		line, err := s.normalizeCartLine(r, strings.TrimSpace(it.ProductID), it.Qty)
		if err != nil {
			s.writeCartLineError(w, r, err)
			return
		}
		normalized = append(normalized, line)
	}
	_ = s.saveCartAndWrite(w, r, customerID, normalized)
}

func (s *Server) normalizeCartLine(r *http.Request, productRef string, qty int) (customers.CartLine, error) {
	if s.Catalog == nil || s.DB == nil {
		return customers.CartLine{}, errCartLineInvalid
	}
	productRef = strings.TrimSpace(productRef)
	if productRef == "" || qty <= 0 {
		return customers.CartLine{}, errCartLineInvalid
	}
	currency := cartCurrencyFromRequest(r)
	p, err := catalog.ResolveProduct(r.Context(), s.Catalog, s.DB, productRef)
	if err != nil {
		return customers.CartLine{}, err
	}
	price, err := s.Catalog.GetCurrentPrice(r.Context(), s.DB, p.ID, currency)
	if err != nil {
		return customers.CartLine{}, err
	}
	return customers.CartLine{
		ProductID:    catalog.PublicID(p.ID),
		SKU:          p.SKU,
		Name:         p.Name,
		Currency:     price.Currency,
		UnitPrice:    price.AmountMinorUnits,
		Qty:          qty,
		AvailableQty: s.availableQty(r.Context(), p.ID),
	}, nil
}

func (s *Server) writeCartLineError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errCartLineInvalid) {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "invalid cart line")
		return
	}
	if err == domain.ErrNotFound {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "invalid_input", "unknown product")
		return
	}
	writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
}

func (s *Server) saveCartAndWrite(w http.ResponseWriter, r *http.Request, customerID domain.CustomerID, stored []customers.CartLine) error {
	if err := s.Customers.SaveCart(r.Context(), customerID, stored); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return err
	}
	view, err := s.buildCartView(r, customerID, stored)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return err
	}
	s.writeCartJSON(w, view)
	return nil
}

func (s *Server) buildCartView(r *http.Request, customerID domain.CustomerID, stored []customers.CartLine) (cart.View, error) {
	if s.Catalog == nil || s.DB == nil {
		view := cart.View{Currency: cartCurrencyFromRequest(r), Items: []cart.ItemView{}}
		return s.attachCartAddress(r, customerID, view)
	}
	avail := func(ctx context.Context, productID domain.ProductID) int {
		return s.availableQty(ctx, productID)
	}
	view, err := cart.BuildView(r.Context(), s.Catalog, s.DB, avail, cartCurrencyFromRequest(r), stored)
	if err != nil {
		return view, err
	}
	return s.attachCartAddress(r, customerID, view)
}

func (s *Server) attachCartAddress(r *http.Request, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	if s.Customers == nil {
		return view, nil
	}
	addrID, err := s.Customers.LoadCartAddressID(r.Context(), customerID)
	if err != nil || addrID == "" || s.Addresses == nil || s.DB == nil {
		return view, err
	}
	a, err := addresses.ResolveAddress(r.Context(), s.Addresses, s.DB, customerID, string(addrID))
	if err != nil {
		if err == domain.ErrNotFound {
			return view, nil
		}
		return view, err
	}
	av := addresses.AddressViewFromDomain(*a)
	view.AddressID = av.ID
	view.Address = &av
	return view, nil
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
