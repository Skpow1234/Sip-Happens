package addresses

import (
	"context"
	"testing"
	"time"

	"sip-happens/pkg/domain"
	"sip-happens/pkg/ids"
	"sip-happens/services/core-api/internal/testutil"
)

func TestCustomerAddresses_MigrationAndCRUD(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("need postgres: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	var n int
	if err := tdb.DB.QueryRowContext(ctx, `
select count(*) from information_schema.tables
where table_schema = current_schema() and table_name = 'customer_addresses'
`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("customer_addresses table missing: n=%d err=%v", n, err)
	}

	custID, _ := ids.UUIDv4()
	fp := "SHA256:addr-migration-test"
	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, full_name, city, neighborhood, address_line1, phone, legal_id, created_at, updated_at)
values ($1,$2,'Test User','Bogotá','Chapinero','Calle 10','300','CC1',now(),now())
`, custID, fp)
	if err != nil {
		t.Fatal(err)
	}

	repo := NewSQLRepository()
	now := time.Now()
	addrID, _ := ids.UUIDv4()
	addr := domain.CustomerAddress{
		ID:             domain.AddressID(addrID),
		CustomerID:     domain.CustomerID(custID),
		Label:          "Casa",
		FullName:       "Test User",
		Phone:          "300",
		LegalID:        "CC1",
		AddressLine1:   "Calle 10 # 20",
		Neighborhood:   "Chapinero",
		City:           "Bogotá",
		CityNormalized: "bogota",
		IsDefault:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.Create(ctx, tdb.DB, addr); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListByCustomer(ctx, tdb.DB, domain.CustomerID(custID))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) < 1 {
		t.Fatal("expected at least one address")
	}

	addr2ID, _ := ids.UUIDv4()
	addr2 := addr
	addr2.ID = domain.AddressID(addr2ID)
	addr2.Label = "Oficina"
	addr2.Neighborhood = "El Poblado"
	addr2.City = "Medellín"
	addr2.CityNormalized = "medellin"
	addr2.IsDefault = true
	if err := repo.Create(ctx, tdb.DB, addr2); err != nil {
		t.Fatal(err)
	}

	list, err = repo.ListByCustomer(ctx, tdb.DB, domain.CustomerID(custID))
	if err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, a := range list {
		if a.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("expected exactly one default, got %d", defaults)
	}

	if err := repo.Delete(ctx, tdb.DB, domain.CustomerID(custID), domain.AddressID(addrID)); err != nil {
		t.Fatal(err)
	}
}
