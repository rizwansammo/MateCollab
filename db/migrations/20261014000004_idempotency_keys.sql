-- Idempotency keys for side-effecting POSTs (API-DESIGN.md §7, DATABASE.md §4.10). Keyed by
-- user and RLS-exempt like the other per-user auth state; rows expire after 24 hours.

-- +goose Up

CREATE TABLE idempotency_keys (
  user_id         uuid NOT NULL,
  key             text NOT NULL CHECK (char_length(key) BETWEEN 1 AND 128),
  org_id          uuid,
  method          text NOT NULL,
  path            text NOT NULL,
  request_hash    bytea NOT NULL,
  state           text NOT NULL CHECK (state IN ('in_progress', 'completed')),
  response_status int,
  response_body   jsonb,
  created_at      timestamptz NOT NULL DEFAULT now(),
  expires_at      timestamptz NOT NULL,
  PRIMARY KEY (user_id, key)
);
CREATE INDEX idempotency_keys_expiry ON idempotency_keys (expires_at);

-- +goose Down

DROP TABLE idempotency_keys;
