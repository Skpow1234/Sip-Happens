package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/shipping"
	"sip-happens/services/core-api/internal/addresses"
	"sip-happens/services/core-api/internal/cart"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	stripepayments "sip-happens/services/core-api/internal/payments/stripe"
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
	case http.MethodDelete:
		s.handleCartDelete(w, r, customerID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCartDelete(w http.ResponseWriter, r *http.Request, customerID domain.CustomerID) {
	if err := s.Customers.ClearCart(r.Context(), customerID); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	s.handleCartGet(w, r, customerID)
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
		return s.enrichCartView(r, customerID, view)
	}
	avail := func(ctx context.Context, productID domain.ProductID) int {
		return s.availableQty(ctx, productID)
	}
	view, err := cart.BuildView(r.Context(), s.Catalog, s.DB, avail, cartCurrencyFromRequest(r), stored)
	if err != nil {
		return view, err
	}
	return s.enrichCartView(r, customerID, view)
}

func (s *Server) enrichCartView(r *http.Request, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	view, err := s.attachCartAddress(r, customerID, view)
	if err != nil {
		return view, err
	}
	view, err = s.attachCartPaymentMethod(r, customerID, view)
	if err != nil {
		return view, err
	}
	return s.attachCartShipping(r.Context(), view), nil
}

// attachCartShipping populates ShippingMinorUnits and TotalMinorUnits using
// the same rate lookup as /checkout and /cart/convert, so the cart preview
// shows the same grand total the customer will actually pay. Without an
// attached address (or with an unknown city / missing rate row) we leave
// shipping at zero and total == subtotal — clients that need shipping
// before they have an address should call /shipping/estimate directly.
//
// Uses the weight-tier table when present (roadmap 140); falls back to
// the flat per-city rate when no tier matches (heavy carts) or the
// product weight is unknown.
func (s *Server) attachCartShipping(ctx context.Context, view cart.View) cart.View {
	if s.DB == nil || view.Address == nil {
		view.TotalMinorUnits = view.SubtotalMinorUnits
		return view
	}
	canon, err := shipping.CanonicalCity(view.Address.City)
	if err != nil {
		view.TotalMinorUnits = view.SubtotalMinorUnits
		return view
	}
	weightItems := make([]shipping.ItemForWeight, 0, len(view.Items))
	for _, it := range view.Items {
		weightItems = append(weightItems, shipping.ItemForWeight{
			ProductID: it.Product.ID,
			Qty:       it.Qty,
		})
	}
	totalWeight, err := shipping.TotalWeightGrams(ctx, s.DB, weightItems)
	if err != nil {
		view.TotalMinorUnits = view.SubtotalMinorUnits
		return view
	}
	amount, err := shipping.LookupAmountMinorForWeight(ctx, s.DB, shipping.CityKey(canon), view.Currency, totalWeight)
	if err != nil {
		view.TotalMinorUnits = view.SubtotalMinorUnits
		return view
	}
	view.ShippingMinorUnits = amount
	view.TotalMinorUnits = view.SubtotalMinorUnits + amount
	return view
}

func (s *Server) attachCartAddress(r *http.Request, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	return s.attachCartAddressCtx(r.Context(), customerID, view)
}

func (s *Server) attachCartAddressCtx(ctx context.Context, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	if s.Customers == nil {
		return view, nil
	}
	addrID, err := s.Customers.LoadCartAddressID(ctx, customerID)
	if err != nil || addrID == "" || s.Addresses == nil || s.DB == nil {
		return view, err
	}
	a, err := addresses.ResolveAddress(ctx, s.Addresses, s.DB, customerID, string(addrID))
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

func (s *Server) attachCartPaymentMethod(r *http.Request, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	return s.attachCartPaymentMethodCtx(r.Context(), customerID, view)
}

func (s *Server) attachCartPaymentMethodCtx(ctx context.Context, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	if s.Customers == nil {
		return view, nil
	}
	pmID, err := s.Customers.LoadCartPaymentMethodID(ctx, customerID)
	if err != nil || pmID == "" {
		return view, err
	}
	view.PaymentMethodID = pmID
	if s.Stripe == nil {
		return view, nil
	}
	stripeCustID, err := s.Customers.StripeCustomerID(ctx, customerID)
	if err != nil && err != domain.ErrNotFound {
		return view, err
	}
	if stripeCustID == "" {
		return view, nil
	}
	card, err := s.Stripe.GetPaymentMethod(ctx, stripepayments.GetPaymentMethodInput{
		StripeCustomerID: stripeCustID,
		PaymentMethodID:  pmID,
	})
	if err != nil {
		if errors.Is(err, stripepayments.ErrPaymentMethodNotFound) || errors.Is(err, stripepayments.ErrPaymentMethodNotOwned) {
			return view, nil
		}
		return view, err
	}
	view.PaymentMethod = card
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
