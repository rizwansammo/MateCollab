-- Account deletion lifecycle (DATABASE.md §7): a deletion request starts a grace period during
-- which the user can still sign in and cancel; afterwards the account is anonymised, never
-- hard-deleted, so content and history stay attributable to "Deleted user".

-- +goose Up

ALTER TABLE users
  ADD COLUMN deletion_requested_at timestamptz,
  ADD COLUMN deletion_scheduled_at timestamptz,
  ADD CONSTRAINT users_deletion_consistent
    CHECK ((deletion_requested_at IS NULL) = (deletion_scheduled_at IS NULL));
CREATE INDEX users_deletion_due ON users (deletion_scheduled_at) WHERE deletion_scheduled_at IS NOT NULL;

-- Anonymises one account whose grace period has ended. It runs as the owner because the
-- account's memberships span organisations. It refuses accounts that are not due and accounts
-- that still own an organisation (ownership must be transferred first). Returns whether the
-- account was anonymised.
-- +goose StatementBegin
CREATE FUNCTION mc_anonymise_user(p_user uuid, p_now timestamptz)
RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
BEGIN
  PERFORM 1 FROM users
  WHERE id = p_user AND status <> 'anonymised' AND deletion_scheduled_at <= p_now
  FOR UPDATE;
  IF NOT FOUND THEN
    RETURN false;
  END IF;
  IF EXISTS (SELECT 1 FROM organisation_memberships WHERE user_id = p_user AND role = 'owner') THEN
    RETURN false;
  END IF;

  UPDATE organisation_memberships
  SET status = 'deactivated', status_changed_at = now(), title = NULL, contact_email = NULL
  WHERE user_id = p_user;
  DELETE FROM sessions WHERE user_id = p_user;
  DELETE FROM auth_tokens WHERE user_id = p_user;
  DELETE FROM user_recovery_codes WHERE user_id = p_user;
  DELETE FROM user_totp_factors WHERE user_id = p_user;
  DELETE FROM user_credentials WHERE user_id = p_user;
  DELETE FROM user_preferences WHERE user_id = p_user;
  DELETE FROM user_emails WHERE user_id = p_user;
  UPDATE users
  SET display_name = 'Deleted user', status = 'anonymised', avatar_key = NULL,
      avatar_version = avatar_version + 1, deletion_requested_at = NULL,
      deletion_scheduled_at = NULL, updated_at = now()
  WHERE id = p_user;
  RETURN true;
END
$$;
-- +goose StatementEnd

-- Accounts whose grace period has ended (for the purge job, which has no user context).
-- +goose StatementBegin
CREATE FUNCTION mc_users_due_for_anonymisation(p_now timestamptz, p_limit int)
RETURNS SETOF uuid
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  SELECT id FROM users
  WHERE deletion_scheduled_at <= p_now AND status <> 'anonymised'
  ORDER BY deletion_scheduled_at
  LIMIT p_limit
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION mc_anonymise_user(uuid, timestamptz) FROM PUBLIC;
REVOKE ALL ON FUNCTION mc_users_due_for_anonymisation(timestamptz, int) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION mc_anonymise_user(uuid, timestamptz) TO mc_app;
GRANT EXECUTE ON FUNCTION mc_users_due_for_anonymisation(timestamptz, int) TO mc_app;

-- +goose Down

DROP FUNCTION mc_users_due_for_anonymisation(timestamptz, int);
DROP FUNCTION mc_anonymise_user(uuid, timestamptz);
DROP INDEX users_deletion_due;
ALTER TABLE users
  DROP CONSTRAINT users_deletion_consistent,
  DROP COLUMN deletion_scheduled_at,
  DROP COLUMN deletion_requested_at;
