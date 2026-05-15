package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"sip-happens/pkg/domain"
)

type cartLineJSON struct {
	ProductID    string `json:"product_id"`
	SKU          string `json:"sku,omitempty"`
	Name         string `json:"name,omitempty"`
	Currency     string `json:"currency,omitempty"`
	UnitPrice    int64  `json:"unit_price_minor_units,omitempty"`
	Qty          int    `json:"qty"`
	AvailableQty int    `json:"available_qty,omitempty"`
}

func (s *Server) loadCustomerCart(ctx context.Context, customerID domain.CustomerID) ([]cartLineJSON, error) {
	if s.DB == nil {
		return nil, nil
	}
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `
select items_json from customer_carts where customer_id=$1
`, string(customerID)).Scan(&raw)
	if err == sql.ErrNoRows {
		return []cartLineJSON{}, nil
	}
	if err != nil {
		return nil, err
	}
	var items []cartLineJSON
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Server) saveCustomerCart(ctx context.Context, customerID domain.CustomerID, items []cartLineJSON) error {
	if s.DB == nil {
		return nil
	}
	b, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
insert into customer_carts (customer_id, items_json, updated_at)
values ($1, $2::jsonb, now())
on conflict (customer_id) do update
set items_json = excluded.items_json, updated_at = now()
`, string(customerID), string(b))
	return err
}

func (s *Server) recordCustomerConsent(ctx context.Context, customerID domain.CustomerID, purpose string) error {
	if s.DB == nil || purpose == "" {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
insert into customer_consents (customer_id, purpose, granted_at, revoked_at)
values ($1, $2, now(), null)
on conflict (customer_id, purpose) do update
set granted_at = now(), revoked_at = null
`, string(customerID), purpose)
	return err
}

func parseBirthDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, domain.ErrInvalidInput
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, domain.ErrInvalidInput
}
