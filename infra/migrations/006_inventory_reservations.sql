-- Soft-hold inventory against an order until paid/cancelled.
create table inventory_reservations (
  order_id text not null,
  product_id text not null,
  qty integer not null,
  status text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (order_id, product_id)
);

create index inventory_reservations_order_active_idx
  on inventory_reservations (order_id)
  where status = 'ACTIVE';

create index inventory_reservations_status_idx
  on inventory_reservations (status);
