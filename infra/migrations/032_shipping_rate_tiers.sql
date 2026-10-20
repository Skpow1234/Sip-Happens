-- Weight-based shipping tiers + product weight (migration 032).
-- DefaultItemWeightGrams = 500 in pkg/shipping/rates.go — keep in sync.
alter table products
  add column weight_grams integer default 500;

create table shipping_rate_tiers (
  city_normalized text not null,
  currency text not null,
  max_weight_grams integer not null,
  amount_minor_units bigint not null,
  primary key (city_normalized, currency, max_weight_grams)
);

-- Bogotá/COP tiers asserted by pkg/shipping/rates_test.go.
insert into shipping_rate_tiers (city_normalized, currency, max_weight_grams, amount_minor_units) values
  ('bogota', 'COP', 2000,  6000),
  ('bogota', 'COP', 6000,  8000),
  ('bogota', 'COP', 15000, 14000);

-- Ensure flat Bogotá fallback matches the overflow / unknown-weight path.
insert into shipping_rates (city_normalized, currency, amount_minor_units)
values ('bogota', 'COP', 8000)
on conflict (city_normalized, currency) do update
set amount_minor_units = excluded.amount_minor_units;
