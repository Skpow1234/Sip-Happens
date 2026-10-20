-- One shipment row per order (fulfillment / carrier handoff).
create table shipments (
  id text primary key,
  order_id text not null,
  carrier text,
  tracking_code text,
  label_path text,
  status text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (order_id)
);

create index shipments_status_created_at_idx on shipments (status, created_at);
