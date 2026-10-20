-- Saved shipping addresses per customer.
create table customer_addresses (
  id text primary key,
  customer_id text not null references customers (id),
  label text not null default '',
  full_name text not null default '',
  phone text not null default '',
  legal_id text not null default '',
  address_line1 text not null default '',
  neighborhood text not null default '',
  city text not null default '',
  city_normalized text not null default '',
  is_default boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index customer_addresses_customer_id_idx
  on customer_addresses (customer_id, is_default desc, created_at);
