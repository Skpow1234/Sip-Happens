-- Flat per-city shipping rates (fallback when no weight tier matches).
-- Bogotá/COP = 8000 is asserted by pkg/shipping/rates_test.go.
-- Medellín/Cali amounts are not present in tracked Go/seed; seeded at 8000
-- as provisional parity so all CanonicalCity keys resolve — operators should
-- tune via a follow-up migration if real rates differ.
create table shipping_rates (
  city_normalized text not null,
  currency text not null,
  amount_minor_units bigint not null,
  primary key (city_normalized, currency)
);

insert into shipping_rates (city_normalized, currency, amount_minor_units) values
  ('bogota',   'COP', 8000),
  ('medellin', 'COP', 8000),
  ('cali',     'COP', 8000);
