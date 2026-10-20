-- Customers identified by SSH fingerprint (SSH-gateway auth).
create table customers (
  id text primary key,
  ssh_fingerprint text not null,
  email text,
  phone text,
  legal_id text,
  full_name text,
  birth_date date,
  address_line1 text,
  neighborhood text,
  city text,
  stripe_customer_id text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (ssh_fingerprint)
);

create index customers_stripe_customer_id_idx on customers (stripe_customer_id)
  where stripe_customer_id is not null;
