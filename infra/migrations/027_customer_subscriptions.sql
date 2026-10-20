-- Recurring product subscriptions (Stripe Subscriptions).
-- interval CHECK matches domain week|month; stripe_subscription_id UNIQUE.
create table customer_subscriptions (
  id text primary key,
  customer_id text not null,
  product_id text not null,
  qty integer not null default 1,
  currency text,
  interval text not null,
  unit_amount_minor_units bigint,
  status text not null,
  stripe_subscription_id text,
  stripe_subscription_item_id text,
  address_id text,
  current_period_start timestamptz,
  current_period_end timestamptz,
  cancelled_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint customer_subscriptions_interval_check
    check (interval in ('week', 'month')),
  unique (stripe_subscription_id)
);

create index customer_subscriptions_customer_id_idx
  on customer_subscriptions (customer_id, created_at desc);
create index customer_subscriptions_status_idx
  on customer_subscriptions (status);
