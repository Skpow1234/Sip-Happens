-- Orders awaiting Stripe payment reconciliation retry.
create table stripe_reconcile_queue (
  order_id text primary key,
  next_attempt_at timestamptz not null default now(),
  attempts integer not null default 0,
  last_error text,
  updated_at timestamptz not null default now()
);

create index stripe_reconcile_queue_next_attempt_at_idx
  on stripe_reconcile_queue (next_attempt_at);
