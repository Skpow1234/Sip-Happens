-- Provider payment rows (Stripe Checkout / PaymentIntent refs).
create table payments (
  id text primary key,
  order_id text not null,
  provider text not null,
  provider_ref text not null,
  status text not null,
  raw_payload text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (provider, provider_ref)
);

create index payments_order_id_idx on payments (order_id);
