-- Time-bounded product prices (minor currency units).
-- Seed/admin use ON CONFLICT DO NOTHING; uniqueness is (product_id, currency, valid_from).
create table prices (
  product_id text not null references products (id),
  currency text not null,
  amount_minor_units bigint not null,
  valid_from timestamptz not null,
  valid_through timestamptz,
  primary key (product_id, currency, valid_from)
);

create index prices_product_currency_valid_from_idx
  on prices (product_id, currency, valid_from desc);
