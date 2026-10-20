-- Transactional outbox (migration 005 per CI / dispatcher).
create table outbox_events (
  id text primary key,
  type text not null,
  occurred_at timestamptz not null,
  aggregate_type text not null,
  aggregate_id text not null,
  version integer not null default 1,
  request_id text,
  payload_json text not null,
  status text not null default 'PENDING',
  attempts integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  locked_at timestamptz,
  locked_by text,
  last_error text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- ClaimPending: status=PENDING AND next_attempt_at <= now() ORDER BY … FOR UPDATE SKIP LOCKED
create index outbox_events_claim_idx
  on outbox_events (status, next_attempt_at, occurred_at);

create table outbox_dead_letters (
  outbox_event_id text primary key,
  type text not null,
  occurred_at timestamptz not null,
  aggregate_type text not null,
  aggregate_id text not null,
  version integer not null,
  request_id text,
  payload_json text not null,
  attempts integer not null,
  last_error text,
  failed_at timestamptz not null default now()
);

-- ReplayDeadLetters: ORDER BY failed_at DESC FOR UPDATE SKIP LOCKED
create index outbox_dead_letters_failed_at_idx on outbox_dead_letters (failed_at desc);
