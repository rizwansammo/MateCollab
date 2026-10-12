-- Global identity (DATABASE.md §4.1, §4.10): users, emails, credentials, second factors,
-- sessions, single-use tokens, preferences and account-level security history.
--
-- users and user_emails carry RLS (own rows; co-members of the current organisation may read
-- user rows). The credential, session and token tables are deliberately RLS-exempt: they are
-- not tenant data and only the auth package reads them (MULTITENANCY.md §2.5).

-- +goose Up

CREATE TABLE users (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  display_name   text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 80),
  avatar_key     text,
  avatar_version int  NOT NULL DEFAULT 0,
  timezone       text NOT NULL DEFAULT 'UTC' CHECK (char_length(timezone) <= 64),
  locale         text NOT NULL DEFAULT 'en-GB' CHECK (char_length(locale) <= 35),
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'anonymised')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX users_name_trgm ON users USING gin (display_name gin_trgm_ops);

CREATE TABLE user_emails (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  email       text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
  is_primary  boolean NOT NULL DEFAULT false,
  verified_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
-- Global uniqueness covers proven addresses only (verified, or primary set at account creation):
-- an unverified secondary claim must not block the real owner from accepting an invitation, and
-- it fails with a unique violation if its holder later tries to verify it.
CREATE UNIQUE INDEX user_emails_email_uq ON user_emails (lower(email)) WHERE verified_at IS NOT NULL OR is_primary;
CREATE UNIQUE INDEX user_emails_user_email_uq ON user_emails (user_id, lower(email));
CREATE UNIQUE INDEX user_emails_primary_uq ON user_emails (user_id) WHERE is_primary;

CREATE TABLE user_credentials (
  user_id             uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  password_hash       text NOT NULL,
  password_changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_totp_factors (
  user_id           uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  secret_ciphertext bytea NOT NULL,        -- keyring Seal, AAD = user_id || 'totp'
  key_id            text  NOT NULL,
  confirmed_at      timestamptz,           -- NULL until the first valid code
  last_used_step    bigint,                -- replay protection
  created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_recovery_codes (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  code_hash  bytea NOT NULL,               -- keyring MAC of the normalised code
  key_id     text  NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_recovery_codes_unused ON user_recovery_codes (user_id) WHERE used_at IS NULL;

CREATE TABLE sessions (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id            uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash         bytea NOT NULL UNIQUE,  -- SHA-256 of the cookie token; the token is never stored
  amr                text[] NOT NULL,        -- authentication methods: {pwd}, {pwd,otp}, {pwd,recovery}, {invite}
  created_at         timestamptz NOT NULL DEFAULT now(),
  last_seen_at       timestamptz NOT NULL DEFAULT now(),
  idle_expires_at    timestamptz NOT NULL,
  expires_at         timestamptz NOT NULL,   -- absolute platform cap
  reauthenticated_at timestamptz NOT NULL DEFAULT now(),
  revoked_at         timestamptz,
  revoked_reason     text CHECK (char_length(revoked_reason) <= 40),
  ip                 inet,
  user_agent         text CHECK (char_length(user_agent) <= 512)
);
CREATE INDEX sessions_user_active ON sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX sessions_expired ON sessions (expires_at);

CREATE TABLE auth_tokens (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid REFERENCES users (id) ON DELETE CASCADE,
  purpose    text NOT NULL CHECK (purpose IN ('email_verification', 'password_reset', 'mfa_login', 'email_change')),
  token_hash bytea NOT NULL UNIQUE,
  data       jsonb NOT NULL DEFAULT '{}',   -- e.g. {"email_id": "…"}; never secrets
  attempts   int   NOT NULL DEFAULT 0,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_tokens_user ON auth_tokens (user_id, purpose) WHERE used_at IS NULL;
CREATE INDEX auth_tokens_expired ON auth_tokens (expires_at);

CREATE TABLE user_preferences (
  user_id        uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
  prefs          jsonb NOT NULL DEFAULT '{}',
  schema_version int   NOT NULL DEFAULT 1,
  updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE account_audit_events (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid REFERENCES users (id) ON DELETE CASCADE,  -- NULL for failures against unknown accounts
  action     text NOT NULL CHECK (char_length(action) <= 64),
  outcome    text NOT NULL CHECK (outcome IN ('success', 'failure')),
  ip         inet,
  user_agent text CHECK (char_length(user_agent) <= 512),
  session_id uuid,
  request_id text CHECK (char_length(request_id) <= 128),
  metadata   jsonb NOT NULL DEFAULT '{}',                   -- never passwords, codes or tokens
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX account_audit_events_user ON account_audit_events (user_id, created_at DESC);

-- Append-only security history: the runtime role may only add and read.
REVOKE UPDATE, DELETE, TRUNCATE ON account_audit_events FROM mc_app;

-- users: own row, or members of the current organisation (global identity fields only).
-- The membership condition is added in the organisations migration, once that table exists.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
CREATE POLICY users_select_self ON users FOR SELECT USING (id = mc_current_user_id());
CREATE POLICY users_insert_self ON users FOR INSERT WITH CHECK (id = mc_current_user_id());
CREATE POLICY users_update_self ON users FOR UPDATE
  USING (id = mc_current_user_id()) WITH CHECK (id = mc_current_user_id());

-- user_emails: the owner only (co-members see memberships.contact_email instead).
ALTER TABLE user_emails ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_emails FORCE ROW LEVEL SECURITY;
CREATE POLICY user_emails_owner ON user_emails
  USING (user_id = mc_current_user_id()) WITH CHECK (user_id = mc_current_user_id());

-- +goose Down

DROP TABLE account_audit_events;
DROP TABLE user_preferences;
DROP TABLE auth_tokens;
DROP TABLE sessions;
DROP TABLE user_recovery_codes;
DROP TABLE user_totp_factors;
DROP TABLE user_credentials;
DROP TABLE user_emails;
DROP TABLE users;
