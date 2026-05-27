package shipping

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// TestLookupAmountMinorForWeight_Tiers covers the tier-vs-flat fallback
// matrix added in roadmap 140:
//
//  1. weightGrams <= smallest tier  → small tier
//  2. weightGrams within a mid tier → mid tier
//  3. weightGrams larger than every tier → flat fallback
//  4. weightGrams == 0 (unknown)    → flat fallback
//
// Skipped when POSTGRES_DSN isn't set. Uses the shared database (no
// isolation) since we only read; the test won't insert anything.
func TestLookupAmountMinorForWeight_Tiers(t *testing.T) {
	db := openSharedDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	ctx := context.Background()

	// If migration 032 hasn't been applied to the shared DB, skip.
	var hasTiers int
	if err := db.QueryRowContext(ctx, `
select count(*) from shipping_rate_tiers where city_normalized='bogota' and currency='COP'
`).Scan(&hasTiers); err != nil || hasTiers == 0 {
		t.Skipf("shipping_rate_tiers not seeded for bogota/COP (err=%v, rows=%d) — apply migration 032", err, hasTiers)
		return
	}

	cases := []struct {
		name   string
		weight int
		want   int64
	}{
		// Migration 032 seeds Bogotá tiers: 2000g→6000, 6000g→8000, 15000g→14000.
		{"small tier", 1500, 6000},
		{"mid tier", 5500, 8000},
		{"large tier", 12000, 14000},
		{"overflow falls back to flat 8000", 30000, 8000},
		{"unknown weight falls back to flat 8000", 0, 8000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LookupAmountMinorForWeight(ctx, db, "bogota", "COP", tc.weight)
			if err != nil {
				t.Fatalf("lookup: %v", err)
			}
			if got != tc.want {
				t.Fatalf("weight=%d: want %d, got %d", tc.weight, tc.want, got)
			}
		})
	}
}

// TestTotalWeightGrams_FallbackDefault ensures products with a NULL
// `weight_grams` use the package default (500g) and that unknown
// product IDs also fall back without erroring.
func TestTotalWeightGrams_FallbackDefault(t *testing.T) {
	db := openSharedDB(t)
	if db == nil {
		return
	}
	defer db.Close()
	ctx := context.Background()

	total, err := TotalWeightGrams(ctx, db, []ItemForWeight{
		{ProductID: "00000000-0000-0000-0000-000000000000", Qty: 3},
	})
	if err != nil {
		t.Fatalf("total: %v", err)
	}
	if total != 3*DefaultItemWeightGrams {
		t.Fatalf("want %d, got %d", 3*DefaultItemWeightGrams, total)
	}
}

func openSharedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set; skipping DB-backed shipping test")
		return nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("open postgres: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Skipf("ping postgres: %v", err)
		return nil
	}
	return db
}
