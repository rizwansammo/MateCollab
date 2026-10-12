-- Organisations, memberships, settings, invitations and organisation audit logs
-- (DATABASE.md §4.2, §4.10; MULTITENANCY.md §2.5), plus the narrow SECURITY DEFINER lookups used
-- before a tenant or user context exists (login, session resolution, invitation acceptance).

-- +goose Up

CREATE TABLE organisations (
  id                  uuid PRIMARY KEY DEFAULT uuidv7(),
  slug                text NOT NULL CHECK (slug ~ '^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$'),
  name                text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  logo_key            text,
  default_timezone    text NOT NULL DEFAULT 'UTC' CHECK (char_length(default_timezone) <= 64),
  default_locale      text NOT NULL DEFAULT 'en-GB' CHECK (char_length(default_locale) <= 35),
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleting')),
  support_request_seq bigint NOT NULL DEFAULT 0,
  storage_used_bytes  bigint NOT NULL DEFAULT 0,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  delete_requested_at timestamptz
);
CREATE UNIQUE INDEX organisations_slug_uq ON organisations (lower(slug));

CREATE TABLE organisation_memberships (
  org_id            uuid NOT NULL REFERENCES organisations (id),
  user_id           uuid NOT NULL REFERENCES users (id),
  role              text NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'guest')),
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deactivated')),
  title             text CHECK (char_length(title) <= 100),
  contact_email     text CHECK (char_length(contact_email) <= 254),
  presence_override text NOT NULL DEFAULT 'auto'
                    CHECK (presence_override IN ('auto', 'busy', 'dnd', 'away', 'invisible')),
  status_text       text CHECK (char_length(status_text) <= 100),
  status_expires_at timestamptz,
  invited_by        uuid,
  joined_at         timestamptz NOT NULL DEFAULT now(),
  status_changed_at timestamptz,
  PRIMARY KEY (org_id, user_id)
);
CREATE UNIQUE INDEX memberships_one_owner ON organisation_memberships (org_id) WHERE role = 'owner';
CREATE INDEX memberships_user ON organisation_memberships (user_id) WHERE status = 'active';

CREATE TABLE organisation_settings (
  org_id         uuid NOT NULL REFERENCES organisations (id),
  section        text NOT NULL CHECK (section IN ('security', 'messaging', 'meetings', 'files', 'notifications', 'support')),
  settings       jsonb NOT NULL,
  schema_version int   NOT NULL DEFAULT 1,
  version        int   NOT NULL DEFAULT 1,
  updated_by     uuid,
  updated_at     timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, section)
);

CREATE TABLE invitations (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id       uuid NOT NULL REFERENCES organisations (id),
  email        text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
  role         text NOT NULL CHECK (role IN ('admin', 'member', 'guest')),
  token_hash   bytea NOT NULL UNIQUE,      -- SHA-256 of the emailed token; rotated on resend
  invited_by   uuid  NOT NULL,
  expires_at   timestamptz NOT NULL,
  last_sent_at timestamptz NOT NULL DEFAULT now(),
  send_count   int   NOT NULL DEFAULT 1,
  accepted_at  timestamptz,
  accepted_by  uuid REFERENCES users (id),
  revoked_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, invited_by) REFERENCES organisation_memberships (org_id, user_id)
);
CREATE UNIQUE INDEX invitations_pending_uq ON invitations (org_id, lower(email))
  WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX invitations_org_created ON invitations (org_id, created_at DESC);

CREATE TABLE audit_logs (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id        uuid NOT NULL REFERENCES organisations (id),
  actor_type    text NOT NULL CHECK (actor_type IN ('user', 'system', 'integration')),
  actor_user_id uuid REFERENCES users (id),
  action        text NOT NULL CHECK (char_length(action) <= 64),
  target_type   text CHECK (char_length(target_type) <= 40),
  target_id     text CHECK (char_length(target_id) <= 64),
  outcome       text NOT NULL DEFAULT 'success' CHECK (outcome IN ('success', 'failure', 'denied')),
  ip            inet,
  user_agent    text CHECK (char_length(user_agent) <= 512),
  session_id    uuid,
  request_id    text CHECK (char_length(request_id) <= 128),
  metadata      jsonb NOT NULL DEFAULT '{}',   -- changed fields; never secrets or content
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_org_time ON audit_logs (org_id, created_at DESC);
CREATE INDEX audit_logs_actor ON audit_logs (org_id, actor_user_id, created_at DESC);
REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM mc_app;

-- ---- Row-level security ------------------------------------------------------------------

ALTER TABLE organisations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organisations FORCE ROW LEVEL SECURITY;
CREATE POLICY organisations_select ON organisations FOR SELECT USING (
  id = mc_current_org_id()
  OR EXISTS (SELECT 1 FROM organisation_memberships m
             WHERE m.org_id = organisations.id AND m.user_id = mc_current_user_id() AND m.status = 'active')
);
CREATE POLICY organisations_insert ON organisations FOR INSERT WITH CHECK (id = mc_current_org_id());
CREATE POLICY organisations_update ON organisations FOR UPDATE
  USING (id = mc_current_org_id()) WITH CHECK (id = mc_current_org_id());

-- A user may list their own memberships across organisations; writes need the org context.
ALTER TABLE organisation_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE organisation_memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY memberships_select ON organisation_memberships FOR SELECT
  USING (org_id = mc_current_org_id() OR user_id = mc_current_user_id());
CREATE POLICY memberships_insert ON organisation_memberships FOR INSERT WITH CHECK (org_id = mc_current_org_id());
CREATE POLICY memberships_update ON organisation_memberships FOR UPDATE
  USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());
CREATE POLICY memberships_delete ON organisation_memberships FOR DELETE USING (org_id = mc_current_org_id());

ALTER TABLE organisation_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE organisation_settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON organisation_settings
  USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());

ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON invitations
  USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit_logs
  USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());

-- Co-members of the current organisation can read each other's global identity fields.
CREATE POLICY users_select_co_members ON users FOR SELECT USING (
  EXISTS (SELECT 1 FROM organisation_memberships m
          WHERE m.user_id = users.id AND m.org_id = mc_current_org_id())
);

-- ---- Pre-context lookups (SECURITY DEFINER, owned by mc_owner, pinned search_path) ----------

-- +goose StatementBegin
CREATE FUNCTION mc_auth_login_lookup(p_email text)
RETURNS TABLE (user_id uuid, user_status text, display_name text, password_hash text, totp_enabled boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  SELECT u.id, u.status, u.display_name, c.password_hash, (t.confirmed_at IS NOT NULL)
  FROM user_emails e
  JOIN users u ON u.id = e.user_id
  LEFT JOIN user_credentials c ON c.user_id = u.id
  LEFT JOIN user_totp_factors t ON t.user_id = u.id
  WHERE lower(e.email) = lower(p_email) AND (e.is_primary OR e.verified_at IS NOT NULL)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION mc_session_lookup(p_token_hash bytea)
RETURNS TABLE (session_id uuid, user_id uuid, amr text[], created_at timestamptz, last_seen_at timestamptz,
               idle_expires_at timestamptz, expires_at timestamptz, reauthenticated_at timestamptz,
               revoked_at timestamptz, user_status text, totp_enabled boolean)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  SELECT s.id, s.user_id, s.amr, s.created_at, s.last_seen_at, s.idle_expires_at, s.expires_at,
         s.reauthenticated_at, s.revoked_at, u.status,
         EXISTS (SELECT 1 FROM user_totp_factors t WHERE t.user_id = s.user_id AND t.confirmed_at IS NOT NULL)
  FROM sessions s JOIN users u ON u.id = s.user_id
  WHERE s.token_hash = p_token_hash
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION mc_invitation_by_token(p_token_hash bytea)
RETURNS TABLE (id uuid, org_id uuid, email text, role text, expires_at timestamptz,
               accepted_at timestamptz, revoked_at timestamptz,
               org_name text, org_slug text, org_status text, inviter_name text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  SELECT i.id, i.org_id, i.email, i.role, i.expires_at, i.accepted_at, i.revoked_at,
         o.name, o.slug, o.status, u.display_name
  FROM invitations i
  JOIN organisations o ON o.id = i.org_id
  LEFT JOIN users u ON u.id = i.invited_by
  WHERE i.token_hash = p_token_hash
$$;
-- +goose StatementEnd

-- Whether an email address already belongs to a member of the current organisation. Only
-- answers for the organisation in the caller's tenant context.
-- +goose StatementBegin
CREATE FUNCTION mc_org_email_is_member(p_org uuid, p_email text)
RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
AS $$
  SELECT coalesce(p_org = mc_current_org_id(), false) AND EXISTS (
    SELECT 1 FROM organisation_memberships m
    JOIN user_emails e ON e.user_id = m.user_id
    WHERE m.org_id = p_org AND m.status <> 'deactivated' AND lower(e.email) = lower(p_email)
      AND (e.is_primary OR e.verified_at IS NOT NULL)
  )
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION mc_auth_login_lookup(text) FROM PUBLIC;
REVOKE ALL ON FUNCTION mc_session_lookup(bytea) FROM PUBLIC;
REVOKE ALL ON FUNCTION mc_invitation_by_token(bytea) FROM PUBLIC;
REVOKE ALL ON FUNCTION mc_org_email_is_member(uuid, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION mc_auth_login_lookup(text) TO mc_app;
GRANT EXECUTE ON FUNCTION mc_session_lookup(bytea) TO mc_app;
GRANT EXECUTE ON FUNCTION mc_invitation_by_token(bytea) TO mc_app;
GRANT EXECUTE ON FUNCTION mc_org_email_is_member(uuid, text) TO mc_app;

-- +goose Down

DROP FUNCTION mc_org_email_is_member(uuid, text);
DROP FUNCTION mc_invitation_by_token(bytea);
DROP FUNCTION mc_session_lookup(bytea);
DROP FUNCTION mc_auth_login_lookup(text);
DROP POLICY users_select_co_members ON users;
DROP POLICY organisations_select ON organisations;
DROP TABLE audit_logs;
DROP TABLE invitations;
DROP TABLE organisation_settings;
DROP TABLE organisation_memberships;
DROP TABLE organisations;
