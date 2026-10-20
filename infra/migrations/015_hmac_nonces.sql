-- Replay protection for HMAC-signed SSH-gateway requests.
create table hmac_nonces (
  fingerprint text not null,
  nonce text not null,
  ts bigint not null,
  expires_at timestamptz not null,
  primary key (fingerprint, nonce, ts)
);

create index hmac_nonces_expires_at_idx on hmac_nonces (expires_at);
