-- Order refunds (idempotent on order_id + idempotency_key).
create table refunds (
  id text primary key,
  order_id text not null,
  payment_id text,
  amount_minor_units bigint not null,
  currency text not null,
  reason text not null default '',
  idempotency_key text not null,
  stripe_refund_id text,
  status text not null,
  actor text not null default '',
  failure_message text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (order_id, idempotency_key)
);

create index refunds_order_id_idx on refunds (order_id);
create index refunds_stripe_refund_id_idx on refunds (stripe_refund_id)
  where stripe_refund_id is not null;
