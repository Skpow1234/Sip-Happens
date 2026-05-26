package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"sip-happens/pkg/domain"
	"sip-happens/services/core-api/internal/cart"
	"sip-happens/services/core-api/internal/catalog"
	"sip-happens/services/core-api/internal/customers"
	"sip-happens/services/core-api/internal/orders"
)

// viewInitDefaults — modest page sizes tuned for the SSH TUI's first paint.
// The TUI can paginate further on demand with /catalog?offset=… / /orders?offset=….
const (
	viewInitCatalogDefaultLimit = 12
	viewInitCatalogMaxLimit     = 100
	viewInitOrdersDefaultLimit  = 5
	viewInitOrdersMaxLimit      = 50
	viewInitFanoutTimeout       = 8 * time.Second
)

// viewInitWarning is a non-fatal partial-failure marker. The TUI uses it to
// show "couldn't load X" without crashing the whole boot.
type viewInitWarning struct {
	Section string `json:"section"`
	Error   string `json:"error"`
}

// viewInitCatalog mirrors the shape of GET /catalog so the TUI can reuse the
// same renderer. omitempty is intentional so a failed catalog section drops
// out of the envelope entirely (the warning carries the diagnostic).
type viewInitCatalog struct {
	Currency string             `json:"currency"`
	Items    []catalog.ItemView `json:"items"`
	Limit    int                `json:"limit"`
	Offset   int                `json:"offset"`
	HasMore  bool               `json:"has_more"`
}

type viewInitOrders struct {
	Orders  []orders.OrderListView `json:"orders"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
	HasMore bool                   `json:"has_more"`
}

// handleViewInit returns one envelope with everything the TUI needs to render
// its first screen after the age gate, fetched concurrently. Profile is the
// only hard requirement — failures in catalog / cart / orders downgrade to a
// `warnings[]` entry so the TUI can still draw a usable shell.
func (s *Server) handleViewInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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

	currency := strings.TrimSpace(r.URL.Query().Get("currency"))
	if currency == "" {
		currency = "COP"
	}
	catalogLimit := parseBoundedQuery(r, "catalog_limit", viewInitCatalogDefaultLimit, viewInitCatalogMaxLimit)
	ordersLimit := parseBoundedQuery(r, "orders_limit", viewInitOrdersDefaultLimit, viewInitOrdersMaxLimit)

	// Bound the fan-out so one slow dependency can't stall the whole init.
	ctx, cancel := context.WithTimeout(r.Context(), viewInitFanoutTimeout)
	defer cancel()

	var (
		mu       sync.Mutex
		warnings []viewInitWarning

		profile     *customers.ProfileView
		catalogPage *viewInitCatalog
		cartView    *cart.View
		ordersPage  *viewInitOrders
	)
	recordWarning := func(section string, err error) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, viewInitWarning{Section: section, Error: err.Error()})
	}

	g, gctx := errgroup.WithContext(ctx)

	// profile — hard dependency; an error here aborts the group via gctx.
	g.Go(func() error {
		c, err := s.Customers.GetByID(gctx, customerID)
		if err != nil {
			return err
		}
		pv := customers.ProfileViewFromDomain(c)
		mu.Lock()
		profile = &pv
		mu.Unlock()
		return nil
	})

	// catalog — soft dependency.
	g.Go(func() error {
		if s.Catalog == nil || s.DB == nil {
			return nil
		}
		page, err := s.Catalog.ListActiveProducts(gctx, s.DB, catalog.ListCatalogFilter{
			Currency: currency,
			Limit:    catalogLimit,
			Offset:   0,
			Featured: true,
		})
		if err != nil {
			recordWarning("catalog", err)
			return nil
		}
		items := make([]catalog.ItemView, 0, len(page.Items))
		for _, it := range page.Items {
			items = append(items, catalog.ItemViewFromProductWithPrice(it))
		}
		mu.Lock()
		catalogPage = &viewInitCatalog{
			Currency: currency,
			Items:    items,
			Limit:    catalogLimit,
			Offset:   0,
			HasMore:  page.HasMore,
		}
		mu.Unlock()
		return nil
	})

	// cart — soft dependency. Reuses the live buildCartView helper so the
	// envelope's `cart` field is byte-identical to GET /cart.
	g.Go(func() error {
		stored, err := s.Customers.LoadCart(gctx, customerID)
		if err != nil {
			recordWarning("cart", err)
			return nil
		}
		view, err := s.buildCartViewWithCtx(gctx, r, customerID, currency, stored)
		if err != nil {
			recordWarning("cart", err)
			return nil
		}
		mu.Lock()
		cartView = &view
		mu.Unlock()
		return nil
	})

	// orders — soft dependency. Mirrors GET /orders' empty-customer behavior
	// (a brand-new caller with no orders still gets {orders:[]} rather than a
	// warning, since "no orders yet" is a normal state for a first session).
	g.Go(func() error {
		if s.Orders == nil {
			return nil
		}
		page, err := s.Orders.ListOrdersByCustomer(gctx, customerID, ordersLimit, 0)
		if err != nil {
			recordWarning("orders", err)
			return nil
		}
		views := make([]orders.OrderListView, 0, len(page.Orders))
		for _, o := range page.Orders {
			views = append(views, orders.OrderListViewFromDomain(o))
		}
		mu.Lock()
		ordersPage = &viewInitOrders{
			Orders:  views,
			Limit:   ordersLimit,
			Offset:  0,
			HasMore: page.HasMore,
		}
		mu.Unlock()
		return nil
	})

	if err := g.Wait(); err != nil {
		// Only a profile failure (or context deadline) can bubble out.
		if errors.Is(err, context.DeadlineExceeded) {
			writeAPIError(w, r, http.StatusGatewayTimeout, "timeout", "view init dependencies timed out")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(w, r, http.StatusNotFound, "not_found", "customer not found")
			return
		}
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	if profile == nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	out := map[string]any{
		"profile":    profile,
		"fetched_at": time.Now().UTC(),
	}
	if catalogPage != nil {
		out["catalog"] = catalogPage
	}
	if cartView != nil {
		out["cart"] = cartView
	}
	if ordersPage != nil {
		out["orders"] = ordersPage
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}

	w.Header().Set("content-type", "application/json")
	w.Header().Set("cache-control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

// buildCartViewWithCtx is buildCartView with an explicit context, used by the
// view-init fan-out where each goroutine carries the errgroup's derived ctx.
func (s *Server) buildCartViewWithCtx(ctx context.Context, r *http.Request, customerID domain.CustomerID, currency string, stored []customers.CartLine) (cart.View, error) {
	if s.Catalog == nil || s.DB == nil {
		view := cart.View{Currency: currency, Items: []cart.ItemView{}}
		return s.enrichCartViewWithCtx(ctx, customerID, view)
	}
	avail := func(ctx context.Context, productID domain.ProductID) int {
		return s.availableQty(ctx, productID)
	}
	view, err := cart.BuildView(ctx, s.Catalog, s.DB, avail, currency, stored)
	if err != nil {
		return view, err
	}
	return s.enrichCartViewWithCtx(ctx, customerID, view)
}

// enrichCartViewWithCtx mirrors enrichCartView but does not touch the request
// once the fan-out's ctx is set. Address / payment attachment failures are
// non-fatal here — the cart still renders without them.
func (s *Server) enrichCartViewWithCtx(ctx context.Context, customerID domain.CustomerID, view cart.View) (cart.View, error) {
	view, _ = s.attachCartAddressCtx(ctx, customerID, view)
	view, _ = s.attachCartPaymentMethodCtx(ctx, customerID, view)
	return view, nil
}

// parseBoundedQuery reads a non-negative integer query param, clamps to
// [1, max], and returns def when the param is missing or invalid.
func parseBoundedQuery(r *http.Request, name string, def, max int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
