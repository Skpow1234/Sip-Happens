-- One cart per customer; line items live in items_json (jsonb).
create table customer_carts (
  customer_id text primary key references customers (id),
  items_json jsonb not null default '[]'::jsonb,
  address_id text,
  payment_method_id text,
  updated_at timestamptz not null default now()
);
