-- Dedup set for processed Stripe webhook event IDs.
create table stripe_webhook_events (
  event_id text primary key,
  processed_at timestamptz not null default now()
);
