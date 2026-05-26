package oauthapps

import (
	"context"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/testutil"
)

func TestMigration_OAuthClients(t *testing.T) {
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
values ('cust-oauth-mig', 'SHA256:oauth-migration-test', now(), now())
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
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris, scopes,
  created_at, updated_at
) values (
  'oc_test_1', 'cust-oauth-mig', 'Test integration',
  'abcdef0123456789', 'feedface00112233', $1,
  array['https://app.example.com/callback']::text[],
  array['catalog:read','orders:read']::text[],
  now(), now()
)
`, hash)
	if err != nil {
		t.Fatalf("insert oauth client: %v", err)
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_blank_name', 'cust-oauth-mig', '   ',
  'abcdef9876543210', 'cafebabe11112222', $1,
  array['https://app.example.com/callback']::text[]
)
`, hash)
	if err == nil {
		t.Fatal("expected name not-blank check to reject blank name")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_no_redirect', 'cust-oauth-mig', 'No redirect',
  'abcdef5555aaaa00', 'cafebabe22223333', $1,
  array[]::text[]
)
`, hash)
	if err == nil {
		t.Fatal("expected redirects-not-empty check to reject empty array")
	}

	shortHash := make([]byte, 16)
	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_short_hash', 'cust-oauth-mig', 'Bad hash',
  'abcdefaaaa55550f', 'deadbeef33334444', $1,
  array['https://app.example.com/callback']::text[]
)
`, shortHash)
	if err == nil {
		t.Fatal("expected hash length check to reject 16-byte hash")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_short_secret_prefix', 'cust-oauth-mig', 'Bad prefix',
  'abcdefbbbb66660f', 'deadbef', $1,
  array['https://app.example.com/callback']::text[]
)
`, hash)
	if err == nil {
		t.Fatal("expected secret prefix length check to reject short prefix")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_dup_client_id', 'cust-oauth-mig', 'Duplicate client_id',
  'abcdef0123456789', 'deadbeef55556666', $1,
  array['https://app.example.com/callback']::text[]
)
`, hash)
	if err == nil {
		t.Fatal("expected unique client_id violation")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_dup_secret_prefix', 'cust-oauth-mig', 'Duplicate secret prefix',
  'abcdef7777cccc00', 'feedface00112233', $1,
  array['https://app.example.com/callback']::text[]
)
`, hash)
	if err == nil {
		t.Fatal("expected unique client_secret_prefix violation")
	}

	_, err = tdb.DB.ExecContext(ctx, `
insert into oauth_clients (
  id, customer_id, name,
  client_id, client_secret_prefix, client_secret_hash,
  redirect_uris
) values (
  'oc_short_client_id', 'cust-oauth-mig', 'Bad client_id',
  'abc', 'deadbeef77778888', $1,
  array['https://app.example.com/callback']::text[]
)
`, hash)
	if err == nil {
		t.Fatal("expected client_id length check to reject short id")
	}
}
