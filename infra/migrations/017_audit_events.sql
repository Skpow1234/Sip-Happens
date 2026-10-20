-- Admin / security audit trail. payload_json is cast ::jsonb in queries.
create table audit_events (
  id bigserial primary key,
  aggregate_type text not null,
  aggregate_id text not null,
  actor text not null,
  event_type text not null,
  payload_json text not null default '',
  created_at timestamptz not null default now()
);

create index audit_events_aggregate_idx
  on audit_events (aggregate_type, aggregate_id, created_at desc);
create index audit_events_created_at_idx on audit_events (created_at desc);
