-- Public newsletter double-opt-in (Ley 1581 / Decreto 1377).
-- token_hash is stored as hex text (see emailsubscribe/service.go), not bytea.
create table email_subscribers (
  id text primary key,
  email text not null,
  status text not null,
  source text not null default 'web',
  consent_ip text,
  consent_user_agent text,
  confirmed_at timestamptz,
  unsubscribed_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (email)
);

create index email_subscribers_status_idx on email_subscribers (status);

create table email_confirmation_tokens (
  id text primary key,
  subscriber_id text not null references email_subscribers (id),
  token_prefix text not null,
  token_hash text not null,
  expires_at timestamptz not null,
  used_at timestamptz,
  created_at timestamptz not null default now()
);

create unique index email_confirmation_tokens_prefix_uidx
  on email_confirmation_tokens (token_prefix);
create index email_confirmation_tokens_subscriber_id_idx
  on email_confirmation_tokens (subscriber_id);
create index email_confirmation_tokens_created_at_idx
  on email_confirmation_tokens (created_at);
