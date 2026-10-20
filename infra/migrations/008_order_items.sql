-- Line items snapshotted at order creation (no separate id column in app SQL).
create table order_items (
  order_id text not null references orders (id),
  product_id text not null,
  sku text not null,
  name text not null,
  qty integer not null,
  unit_price_minor_units bigint not null,
  currency text not null,
  line_total_minor_units bigint not null
);

create index order_items_order_id_idx on order_items (order_id);
