-- Customer personal access tokens (PAT).
-- Constraints enforced by tokens/migration_test.go:
--   name not blank/whitespace; token_hash bytea length 32;
--   token_prefix UNIQUE and length >= 16 (test uses 16-hex; generator emits 32-hex).
create table personal_access_tokens (
  id text primary key,
  customer_id text not null references customers (id),
  name text not null,
  token_prefix text not null,
  token_hash bytea not null,
  scopes text[] not null default '{}'::text[],
  last_used_at timestamptz,
  expires_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint personal_access_tokens_name_not_blank
    check (btrim(name) <> ''),
  constraint personal_access_tokens_token_hash_len
    check (octet_length(token_hash) = 32),
  constraint personal_access_tokens_token_prefix_len
    check (char_length(token_prefix) >= 16),
  unique (token_prefix)
);

create index personal_access_tokens_customer_id_idx
  on personal_access_tokens (customer_id);
create index personal_access_tokens_lookup_idx
  on personal_access_tokens (token_prefix)
  where revoked_at is null;
