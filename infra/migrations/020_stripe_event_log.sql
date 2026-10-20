-- Detailed Stripe webhook intake / processing log.
create table stripe_event_log (
  event_id text primary key,
  event_type text not null,
  order_id text,
  received_at timestamptz not null default now(),
  status text not null default 'RECEIVED',
  raw_payload text not null default '',
  processed_at timestamptz,
  last_error text
);

create index stripe_event_log_order_id_idx on stripe_event_log (order_id)
  where order_id is not null;
create index stripe_event_log_status_received_at_idx
  on stripe_event_log (status, received_at desc);
