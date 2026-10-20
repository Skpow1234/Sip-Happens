-- 18+ affirmation records captured at checkout / cart convert.
create table age_verifications (
  id text primary key,
  customer_id text not null,
  order_id text not null,
  birth_date date not null,
  affirmed_at timestamptz not null,
  method text not null,
  created_at timestamptz not null default now()
);

create index age_verifications_customer_id_idx on age_verifications (customer_id);
create index age_verifications_order_id_idx on age_verifications (order_id);
