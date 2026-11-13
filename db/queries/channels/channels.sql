-- Phase 3 channel conversations. Public channels derive access from Space membership.

-- name: CreateChannel :one
INSERT INTO conversations (id, org_id, kind, space_id, name, topic, visibility, is_default, created_by)
VALUES (@id, @org_id, 'channel', @space_id, @name, @topic, @visibility, @is_default, @created_by)
RETURNING id, org_id, space_id, name, topic, visibility, is_default, created_by, created_at, updated_at, archived_at;

-- name: ChannelByID :one
SELECT c.id, c.org_id, c.space_id, c.name, c.topic, c.visibility, c.is_default, c.created_by,
       c.created_at, c.updated_at, c.archived_at,
       cm.user_id AS viewer_member
FROM conversations c
LEFT JOIN conversation_members cm ON cm.org_id = c.org_id AND cm.conversation_id = c.id
  AND cm.user_id = @viewer_id AND cm.left_at IS NULL
WHERE c.org_id = @org_id AND c.space_id = @space_id AND c.id = @channel_id AND c.kind = 'channel';

-- name: VisibleChannels :many
SELECT c.id, c.org_id, c.space_id, c.name, c.topic, c.visibility, c.is_default, c.created_by,
       c.created_at, c.updated_at, c.archived_at,
       cm.user_id AS viewer_member
FROM conversations c
LEFT JOIN conversation_members cm ON cm.org_id = c.org_id AND cm.conversation_id = c.id
  AND cm.user_id = @viewer_id AND cm.left_at IS NULL
WHERE c.org_id = @org_id AND c.space_id = @space_id AND c.kind = 'channel'
  AND c.archived_at IS NULL AND (c.visibility = 'public' OR cm.user_id IS NOT NULL)
ORDER BY c.is_default DESC, lower(c.name), c.id LIMIT 201;

-- name: UpdateChannel :one
UPDATE conversations SET name = @name, topic = @topic, visibility = @visibility, updated_at = now()
WHERE org_id = @org_id AND space_id = @space_id AND id = @channel_id AND kind = 'channel'
  AND archived_at IS NULL AND NOT is_default AND updated_at = @expected_updated_at
RETURNING id, org_id, space_id, name, topic, visibility, is_default, created_by, created_at, updated_at, archived_at;

-- name: ArchiveChannel :execrows
UPDATE conversations SET archived_at = now(), updated_at = now()
WHERE org_id = @org_id AND space_id = @space_id AND id = @channel_id
  AND kind = 'channel' AND NOT is_default AND archived_at IS NULL;

-- name: AddChannelMember :execrows
INSERT INTO conversation_members (org_id, conversation_id, user_id)
SELECT @org_id, c.id, sm.user_id
FROM conversations c
JOIN space_members sm ON sm.org_id = c.org_id AND sm.space_id = c.space_id AND sm.user_id = @user_id
JOIN organisation_memberships m ON m.org_id = sm.org_id AND m.user_id = sm.user_id AND m.status = 'active'
JOIN spaces s ON s.org_id = c.org_id AND s.id = c.space_id AND s.archived_at IS NULL
WHERE c.org_id = @org_id AND c.space_id = @space_id AND c.id = @channel_id
  AND c.kind = 'channel' AND c.visibility = 'private' AND c.archived_at IS NULL
ON CONFLICT (conversation_id, user_id) DO UPDATE SET left_at = NULL;

-- name: RemoveChannelMember :execrows
UPDATE conversation_members SET left_at = now()
WHERE org_id = @org_id AND conversation_id = @channel_id AND user_id = @user_id AND left_at IS NULL;

-- name: ChannelMembers :many
SELECT cm.user_id, u.display_name, cm.joined_at
FROM conversation_members cm JOIN users u ON u.id = cm.user_id
JOIN organisation_memberships m ON m.org_id = cm.org_id AND m.user_id = cm.user_id AND m.status = 'active'
WHERE cm.org_id = @org_id AND cm.conversation_id = @channel_id AND cm.left_at IS NULL
ORDER BY lower(u.display_name), cm.user_id LIMIT 201;

-- name: ArchiveSpaceChannels :exec
UPDATE conversations SET archived_at = now(), updated_at = now()
WHERE org_id = @org_id AND space_id = @space_id AND kind = 'channel' AND archived_at IS NULL;

-- name: RemoveSpaceChannelMemberships :exec
UPDATE conversation_members cm SET left_at = now()
FROM conversations c WHERE cm.org_id = @org_id AND c.org_id = cm.org_id AND c.id = cm.conversation_id
 AND c.space_id = @space_id AND cm.user_id = @user_id AND c.kind = 'channel'
 AND cm.left_at IS NULL;
