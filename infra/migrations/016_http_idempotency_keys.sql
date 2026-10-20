-- Cached HTTP responses keyed by Idempotency-Key + request shape.
create table http_idempotency_keys (
  scope text not null,
  idem_key text not null,
  method text not null,
  path text not null,
  request_sha text not null,
  status_code integer not null,
  content_type text,
  response_body bytea not null,
  expires_at timestamptz not null,
  primary key (scope, idem_key, method, path)
);

create index http_idempotency_keys_expires_at_idx on http_idempotency_keys (expires_at);
