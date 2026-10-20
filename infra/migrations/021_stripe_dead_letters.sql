-- Stripe webhooks that failed terminal processing (admin replay queue).
create table stripe_dead_letters (
  id bigserial primary key,
  event_id text,
  reason text not null,
  raw_payload text not null default '',
  created_at timestamptz not null default now()
);

create index stripe_dead_letters_created_at_idx on stripe_dead_letters (created_at desc);
