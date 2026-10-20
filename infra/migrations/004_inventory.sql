-- Per-product stock counters (locked with FOR UPDATE during reserve/commit).
create table inventory (
  product_id text primary key references products (id),
  on_hand integer not null default 0,
  reserved integer not null default 0,
  safety_stock integer not null default 0,
  updated_at timestamptz not null default now()
);
