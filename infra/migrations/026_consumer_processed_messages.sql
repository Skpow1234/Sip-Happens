-- NATS / async consumer idempotency (message processed once per consumer).
create table consumer_processed_messages (
  consumer_name text not null,
  message_id text not null,
  processed_at timestamptz not null default now(),
  primary key (consumer_name, message_id)
);
