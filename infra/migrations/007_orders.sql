-- Customer orders (checkout + fulfillment lifecycle).
create table orders (
  id text primary key,
  display_ref text not null,
  status text not null,
  customer_id text not null,
  subtotal_minor_units bigint not null default 0,
  shipping_minor_units bigint not null default 0,
  total_minor_units bigint not null default 0,
  currency text not null,
  city text,
  neighborhood text,
  address_line1 text,
  phone text,
  legal_id text,
  full_name text,
  stripe_checkout_session_id text,
  stripe_payment_intent_id text,
  expires_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (display_ref)
);

create index orders_customer_created_at_idx on orders (customer_id, created_at desc);
create index orders_status_expires_at_idx on orders (status, expires_at);
create index orders_stripe_checkout_session_id_idx
  on orders (stripe_checkout_session_id)
  where stripe_checkout_session_id is not null;
create index orders_stripe_payment_intent_id_idx
  on orders (stripe_payment_intent_id)
  where stripe_payment_intent_id is not null;
