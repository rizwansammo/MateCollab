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

-- Each user sees only their own keys (the API sets mc.user_id); org_id is informational.
ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY user_isolation ON idempotency_keys
  USING (user_id = mc_current_user_id()) WITH CHECK (user_id = mc_current_user_id());

-- Expired keys of all users are removed by the hourly cleanup job, which has no user context.
-- +goose StatementBegin
CREATE FUNCTION mc_purge_idempotency_keys(p_now timestamptz)
RETURNS bigint
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  WITH gone AS (DELETE FROM idempotency_keys WHERE expires_at < p_now RETURNING 1)
  SELECT count(*) FROM gone
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION mc_purge_idempotency_keys(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION mc_purge_idempotency_keys(timestamptz) TO mc_app;

-- +goose Down

DROP FUNCTION mc_purge_idempotency_keys(timestamptz);
DROP TABLE idempotency_keys;
