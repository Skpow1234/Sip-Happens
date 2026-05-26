package tokens

import (
	"context"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/testutil"
)

func TestMigration_PersonalAccessTokens(t *testing.T) {
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
values ('cust-pat-mig', 'SHA256:pat-migration-test', now(), now())
on conflict do nothing
`)
	if err != nil {
		t.Fatal(err)
	}

	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into personal_access_tokens (
  id, customer_id, name, token_prefix, token_hash, scopes, created_at, updated_at
) values (
  'pat_test_1', 'cust-pat-mig', 'ci script', 'abcdef0123456789',
  $1, array['catalog:read','orders:read']::text[], now(), now()
)
`, hash)
	if err != nil {
		t.Fatalf("insert pat: %v", err)
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into personal_access_tokens (
  id, customer_id, name, token_prefix, token_hash
) values (
  'pat_blank_name', 'cust-pat-mig', '   ', 'feedface00000000', $1
)
`, hash)
	if err == nil {
		t.Fatal("expected name not-blank check to reject blank name")
	}

	shortHash := make([]byte, 16)
	_, err = tdb.DB.ExecContext(ctx, `
insert into personal_access_tokens (
  id, customer_id, name, token_prefix, token_hash
) values (
  'pat_short_hash', 'cust-pat-mig', 'bad hash', 'deadbeef11112222', $1
)
`, shortHash)
	if err == nil {
		t.Fatal("expected hash length check to reject 16-byte hash")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into personal_access_tokens (
  id, customer_id, name, token_prefix, token_hash
) values (
  'pat_dup_prefix', 'cust-pat-mig', 'duplicate prefix', 'abcdef0123456789', $1
)
`, hash)
	if err == nil {
		t.Fatal("expected unique token_prefix violation")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into personal_access_tokens (
  id, customer_id, name, token_prefix, token_hash
) values (
  'pat_short_prefix', 'cust-pat-mig', 'short prefix', 'abc', $1
)
`, hash)
	if err == nil {
		t.Fatal("expected prefix length check to reject short prefix")
	}
}
