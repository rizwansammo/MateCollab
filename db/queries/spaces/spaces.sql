-- Spaces are scoped to an organisation, with membership-based private visibility.

-- name: CreateSpace :one
INSERT INTO spaces (id, org_id, name, description, overview, kind, visibility, created_by)
VALUES (@id, @org_id, @name, @description, @overview, @kind, @visibility, @created_by)
RETURNING id, org_id, name, description, overview, kind, visibility, created_by, created_at, updated_at, archived_at;

-- name: SpaceByID :one
SELECT s.id, s.org_id, s.name, s.description, s.overview, s.kind, s.visibility, s.created_by,
       s.created_at, s.updated_at, s.archived_at, sm.role AS viewer_role
FROM spaces s
LEFT JOIN space_members sm ON sm.org_id = s.org_id AND sm.space_id = s.id AND sm.user_id = @viewer_id
WHERE s.org_id = @org_id AND s.id = @space_id;

-- name: VisibleSpaces :many
SELECT s.id, s.org_id, s.name, s.description, s.overview, s.kind, s.visibility, s.created_by,
       s.created_at, s.updated_at, s.archived_at, sm.role AS viewer_role
FROM spaces s
LEFT JOIN space_members sm ON sm.org_id = s.org_id AND sm.space_id = s.id AND sm.user_id = @viewer_id
WHERE s.org_id = @org_id AND s.archived_at IS NULL
  AND (sm.user_id IS NOT NULL OR (@can_manage::boolean) OR (s.visibility = 'public' AND NOT @is_guest::boolean))
ORDER BY lower(s.name), s.id LIMIT 201;

-- name: AddSpaceMember :execrows
INSERT INTO space_members (org_id, space_id, user_id, role, added_by)
SELECT @org_id, @space_id, m.user_id, @role, @added_by
FROM organisation_memberships m
JOIN spaces s ON s.org_id = m.org_id AND s.id = @space_id
WHERE m.org_id = @org_id AND m.user_id = @user_id AND m.status = 'active' AND s.archived_at IS NULL
ON CONFLICT (space_id, user_id) DO UPDATE SET role = EXCLUDED.role, added_by = EXCLUDED.added_by;

-- name: RemoveSpaceMember :execrows
DELETE FROM space_members WHERE org_id = @org_id AND space_id = @space_id AND user_id = @user_id;

-- name: SpaceAdminCount :one
SELECT count(*) FROM space_members WHERE org_id = @org_id AND space_id = @space_id AND role = 'space_admin';

-- name: SpaceMembers :many
SELECT sm.user_id, sm.role, sm.joined_at, u.display_name
FROM space_members sm
JOIN users u ON u.id = sm.user_id
JOIN organisation_memberships m ON m.org_id = sm.org_id AND m.user_id = sm.user_id AND m.status = 'active'
WHERE sm.org_id = @org_id AND sm.space_id = @space_id
ORDER BY lower(u.display_name), sm.user_id LIMIT 201;

-- name: SpaceMemberByID :one
SELECT sm.role FROM space_members sm
WHERE sm.org_id = @org_id AND sm.space_id = @space_id AND sm.user_id = @user_id;

-- name: UpdateSpace :one
UPDATE spaces SET name = @name, description = @description, overview = @overview,
  kind = @kind, visibility = @visibility, updated_at = now()
WHERE org_id = @org_id AND id = @space_id AND archived_at IS NULL
  AND updated_at = @expected_updated_at
RETURNING id, org_id, name, description, overview, kind, visibility, created_by, created_at, updated_at, archived_at;

-- name: ArchiveSpace :execrows
UPDATE spaces SET archived_at = now(), updated_at = now()
WHERE org_id = @org_id AND id = @space_id AND archived_at IS NULL;

-- name: SpaceNameExists :one
SELECT EXISTS(SELECT 1 FROM spaces WHERE org_id = @org_id AND lower(name) = lower(@name)
              AND archived_at IS NULL AND id <> @except_id::uuid);
