-- Spaces, channel conversations and guest Space grants. Phase 3.
-- All tenant tables use enforced RLS and composite tenant-safe references.
-- +goose Up

CREATE TABLE spaces (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id      uuid NOT NULL REFERENCES organisations (id),
  name        text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  description text CHECK (char_length(description) <= 500),
  overview    text CHECK (char_length(overview) <= 5000),
  kind        text NOT NULL DEFAULT 'team' CHECK (kind IN ('team', 'project')),
  visibility  text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
  created_by  uuid NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES organisation_memberships (org_id, user_id)
);
CREATE UNIQUE INDEX spaces_name_uq ON spaces (org_id, lower(name)) WHERE archived_at IS NULL;
CREATE INDEX spaces_name_trgm ON spaces USING gin (name gin_trgm_ops);

CREATE TABLE space_members (
  org_id    uuid NOT NULL,
  space_id  uuid NOT NULL,
  user_id   uuid NOT NULL,
  role      text NOT NULL DEFAULT 'member' CHECK (role IN ('space_admin','member')),
  added_by  uuid,
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (space_id, user_id),
  FOREIGN KEY (org_id, space_id) REFERENCES spaces (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id) REFERENCES organisation_memberships (org_id, user_id),
  FOREIGN KEY (org_id, added_by) REFERENCES organisation_memberships (org_id, user_id)
);
CREATE INDEX space_members_user ON space_members (org_id, user_id);

CREATE TABLE invitation_space_grants (
  org_id        uuid NOT NULL,
  invitation_id uuid NOT NULL,
  space_id      uuid NOT NULL,
  PRIMARY KEY (invitation_id, space_id),
  FOREIGN KEY (org_id, invitation_id) REFERENCES invitations (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, space_id) REFERENCES spaces (org_id, id) ON DELETE CASCADE
);
CREATE INDEX invitation_space_grants_org ON invitation_space_grants (org_id, space_id);

CREATE TABLE conversations (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  org_id           uuid NOT NULL REFERENCES organisations (id),
  kind             text NOT NULL CHECK (kind IN ('dm','group','channel','meeting')),
  space_id         uuid,
  name             text CHECK (char_length(name) <= 80),
  topic            text CHECK (char_length(topic) <= 250),
  visibility       text CHECK (visibility IN ('public', 'private')),
  is_default       boolean NOT NULL DEFAULT false,
  dm_key           text,
  created_by       uuid,
  last_message_seq bigint NOT NULL DEFAULT 0,
  last_change_seq  bigint NOT NULL DEFAULT 0,
  last_message_at  timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  archived_at      timestamptz,
  UNIQUE (org_id, id),
  FOREIGN KEY (org_id, space_id) REFERENCES spaces (org_id, id),
  FOREIGN KEY (org_id, created_by) REFERENCES organisation_memberships (org_id, user_id),
  CHECK ((kind = 'channel') = (space_id IS NOT NULL)),
  CHECK (kind <> 'channel' OR (name IS NOT NULL AND visibility IS NOT NULL)),
  CHECK ((kind = 'dm') = (dm_key IS NOT NULL)),
  CHECK (NOT is_default OR (kind = 'channel' AND visibility = 'public'))
);
CREATE UNIQUE INDEX conversations_dm_uq ON conversations (org_id, dm_key) WHERE kind = 'dm';
CREATE UNIQUE INDEX conversations_channel_uq ON conversations (space_id, lower(name)) WHERE kind = 'channel' AND archived_at IS NULL;
CREATE UNIQUE INDEX conversations_default_uq ON conversations (space_id) WHERE is_default;
CREATE INDEX conversations_space ON conversations (org_id, space_id) WHERE kind = 'channel';
CREATE INDEX conversations_name_trgm ON conversations USING gin (name gin_trgm_ops) WHERE name IS NOT NULL;

CREATE TABLE conversation_members (
  org_id             uuid NOT NULL,
  conversation_id    uuid NOT NULL,
  user_id            uuid NOT NULL,
  joined_at          timestamptz NOT NULL DEFAULT now(),
  left_at            timestamptz,
  last_read_seq      bigint NOT NULL DEFAULT 0,
  last_read_at       timestamptz,
  email_notified_seq bigint NOT NULL DEFAULT 0,
  notification_level text CHECK (notification_level IN ('all','mentions','none')),
  muted_until        timestamptz,
  is_favorite        boolean NOT NULL DEFAULT false,
  hidden_at          timestamptz,
  PRIMARY KEY (conversation_id, user_id),
  FOREIGN KEY (org_id, conversation_id) REFERENCES conversations (org_id, id) ON DELETE CASCADE,
  FOREIGN KEY (org_id, user_id) REFERENCES organisation_memberships (org_id, user_id)
);
CREATE INDEX conversation_members_user ON conversation_members (org_id, user_id) WHERE left_at IS NULL;

ALTER TABLE spaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE spaces FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON spaces USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());
ALTER TABLE space_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE space_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON space_members USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());
ALTER TABLE invitation_space_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE invitation_space_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON invitation_space_grants USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversations USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());
ALTER TABLE conversation_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_members USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON spaces, space_members, invitation_space_grants, conversations, conversation_members TO mc_app;

-- Existing organisations receive an explicit policy section; new organisations seed one during creation.
INSERT INTO organisation_settings (org_id, section, settings)
SELECT id, 'messaging', '{"public_space_creation":"all","private_space_creation":"all"}'::jsonb
FROM organisations ON CONFLICT (org_id, section) DO NOTHING;

-- +goose Down
DROP TABLE conversation_members;
DROP TABLE conversations;
DROP TABLE invitation_space_grants;
DROP TABLE space_members;
DROP TABLE spaces;
