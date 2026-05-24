package subscriptions

import (
	"context"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/testutil"
)

func TestMigration_CustomerSubscriptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("skipping: %v", err)
		return
	}
	defer func() { _ = tdb.CloseFn() }()

	_, err = tdb.DB.ExecContext(ctx, `
insert into customers (id, ssh_fingerprint, created_at, updated_at)
values ('cust-sub-mig', 'SHA256:sub-migration-test', now(), now())
on conflict do nothing
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tdb.DB.ExecContext(ctx, `
insert into products (id, sku, name, brewery, style, abv, volume_ml, invima_reg, is_active, created_at, updated_at)
values ('prod-sub-mig', 'SKU-SUB', 'Sub Beer', 'Brewery', 'Lager', 5.0, 330, 'INVIMA-SUB', true, now(), now())
on conflict do nothing
`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into customer_subscriptions (
  id, customer_id, product_id, qty, currency, interval, unit_amount_minor_units, status,
  stripe_subscription_id, created_at, updated_at
) values (
  'sub_test_1', 'cust-sub-mig', 'prod-sub-mig', 2, 'COP', 'month', 15000, 'ACTIVE',
  'sub_stripe_test_1', now(), now()
)
`)
	if err != nil {
		t.Fatalf("insert subscription: %v", err)
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into customer_subscriptions (id, customer_id, product_id, qty, interval, status)
values ('sub_bad_interval', 'cust-sub-mig', 'prod-sub-mig', 1, 'year', 'ACTIVE')
`)
	if err == nil {
		t.Fatal("expected interval check to reject year")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into customer_subscriptions (
  id, customer_id, product_id, qty, interval, status, stripe_subscription_id
) values (
  'sub_dup', 'cust-sub-mig', 'prod-sub-mig', 1, 'month', 'PENDING', 'sub_stripe_test_1'
)
`)
	if err == nil {
		t.Fatal("expected unique stripe_subscription_id violation")
	}
}
