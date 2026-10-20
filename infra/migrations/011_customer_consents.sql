-- Marketing / terms consents (upsert by customer + purpose).
create table customer_consents (
  customer_id text not null references customers (id),
  purpose text not null,
  granted_at timestamptz not null default now(),
  revoked_at timestamptz,
  primary key (customer_id, purpose)
);
