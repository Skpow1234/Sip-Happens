-- Append-only payment / refund status transitions.
create table payment_status_history (
  id bigserial primary key,
  order_id text not null,
  payment_id text,
  provider text not null,
  provider_ref text not null default '',
  status text not null,
  source text not null,
  detail text,
  created_at timestamptz not null default now()
);

create index payment_status_history_order_id_idx
  on payment_status_history (order_id, created_at desc);
