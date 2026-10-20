-- OAuth clients + authorization codes.
-- Constraints enforced by oauthapps/migration_test.go:
--   name not blank; client_id UNIQUE length >= 16;
--   client_secret_prefix UNIQUE length >= 16;
--   client_secret_hash bytea length 32; redirect_uris non-empty.
create table oauth_clients (
  id text primary key,
  customer_id text not null references customers (id),
  name text not null,
  client_id text not null,
  client_secret_prefix text not null,
  client_secret_hash bytea not null,
  redirect_uris text[] not null,
  scopes text[] not null default '{}'::text[],
  confidential boolean not null default true,
  revoked_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint oauth_clients_name_not_blank
    check (btrim(name) <> ''),
  constraint oauth_clients_client_id_len
    check (char_length(client_id) >= 16),
  constraint oauth_clients_secret_prefix_len
    check (char_length(client_secret_prefix) >= 16),
  constraint oauth_clients_secret_hash_len
    check (octet_length(client_secret_hash) = 32),
  constraint oauth_clients_redirect_uris_not_empty
    check (cardinality(redirect_uris) > 0),
  unique (client_id),
  unique (client_secret_prefix)
);

create index oauth_clients_customer_id_idx on oauth_clients (customer_id);
create index oauth_clients_client_id_active_idx
  on oauth_clients (client_id)
  where revoked_at is null;

create table oauth_authorization_codes (
  id text primary key,
  oauth_client_id text not null references oauth_clients (id),
  customer_id text not null,
  code_prefix text not null,
  code_hash bytea not null,
  redirect_uri text not null,
  scopes text[] not null default '{}'::text[],
  expires_at timestamptz not null,
  consumed_at timestamptz,
  created_at timestamptz not null default now(),
  unique (code_prefix)
);

create index oauth_authorization_codes_consume_idx
  on oauth_authorization_codes (code_prefix)
  where consumed_at is null;
