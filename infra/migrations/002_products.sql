-- Catalog products (craft beer SKUs).
create table products (
  id text primary key,
  sku text not null,
  name text not null,
  brewery text not null default '',
  style text not null default '',
  abv double precision not null default 0,
  volume_ml integer not null default 0,
  invima_reg text not null default '',
  image_url text,
  description text not null default '',
  ibu integer,
  is_active boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (sku)
);

create index products_is_active_created_at_idx on products (is_active, created_at desc);
