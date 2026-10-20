-- Electronic invoice rows (DIAN columns added in migration 030).
create table invoices (
  id text primary key,
  order_id text not null,
  provider text not null,
  external_id text not null default '',
  status text not null,
  payload_json jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (order_id)
);
