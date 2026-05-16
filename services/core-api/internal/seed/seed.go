package seed

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"sip-happens/pkg/ids"
)

type DemoSeedOutput struct {
	CustomerID      string
	SSHFingerprint  string
	ProductIDs      []string
	DemoSSHPubKey   string // path used when present
}

// Demo inserts a minimal dataset required to run a prototype checkout:
// - one customer
// - a few products (active, with INVIMA id)
// - COP prices
// - inventory rows
func Demo(ctx context.Context, db *sql.DB) (*DemoSeedOutput, error) {
	now := time.Now()

	customerID, err := ids.UUIDv4()
	if err != nil {
		return nil, err
	}

	fingerprint, err := DemoSSHFingerprint()
	if err != nil {
		return nil, err
	}
	pubPath := ""
	if _, err := loadDemoPublicKeyBytes(); err == nil {
		for _, p := range demoPublicKeyPaths() {
			if _, statErr := os.Stat(p); statErr == nil {
				pubPath = p
				break
			}
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Customer (upsert-ish for repeatable seeds)
	_, err = tx.ExecContext(ctx, `
	insert into customers (id, ssh_fingerprint, email, phone, legal_id, full_name, birth_date, address_line1, neighborhood, city, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
on conflict (ssh_fingerprint) do update
set email=excluded.email, full_name=excluded.full_name, updated_at=excluded.updated_at
`, customerID, fingerprint, "demo@example.com", "+57 300 000 0000", "CC-0000000", "Demo Customer", "1995-01-01", "Calle 1 # 2-3", "Chapinero", "Bogotá", now, now)
	if err != nil {
		return nil, fmt.Errorf("insert customer: %w", err)
	}

	var idsOut []string
	for _, p := range demoProducts() {
		pid, err := ids.UUIDv4()
		if err != nil {
			return nil, err
		}

		err = tx.QueryRowContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, description, ibu, is_active, created_at, updated_at)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,true,$11,$12)
on conflict (sku) do update set
  name=excluded.name,
  brewery=excluded.brewery,
  style=excluded.style,
  abv=excluded.abv,
  volume_ml=excluded.volume_ml,
  invima_reg=excluded.invima_reg,
  description=excluded.description,
  ibu=excluded.ibu,
  is_active=true,
  updated_at=excluded.updated_at
returning id
`, pid, p.SKU, p.Name, p.Brewery, p.Style, p.ABV, p.VolumeML, p.Invima, p.Description, p.IBU, now, now).Scan(&pid)
		if err != nil {
			return nil, fmt.Errorf("insert product %s: %w", p.SKU, err)
		}

		// Price: use valid_from = now; allow multiple seed runs by making PK unique by (product_id, valid_from)
		_, err = tx.ExecContext(ctx, `
insert into prices (product_id, currency, amount_minor_units, valid_from)
values ($1,'COP',$2,$3)
on conflict do nothing
`, pid, p.PriceCOP, now)
		if err != nil {
			return nil, fmt.Errorf("insert price %s: %w", p.SKU, err)
		}

		// Inventory: if row already exists, update on_hand (keep reserved/safety as-is)
		_, err = tx.ExecContext(ctx, `
insert into inventory (product_id, on_hand, reserved, safety_stock, updated_at)
values ($1,$2,0,0,now())
on conflict (product_id) do update set on_hand=excluded.on_hand, updated_at=now()
`, pid, p.OnHand)
		if err != nil {
			return nil, fmt.Errorf("upsert inventory %s: %w", p.SKU, err)
		}

		idsOut = append(idsOut, pid)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &DemoSeedOutput{
		CustomerID:     customerID,
		SSHFingerprint: fingerprint,
		ProductIDs:     idsOut,
		DemoSSHPubKey:  pubPath,
	}, nil
}

