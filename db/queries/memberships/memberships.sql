-- Memberships module: organisation membership, roles and status. Tenant queries always carry
-- an explicit org_id predicate in addition to RLS (MULTITENANCY.md §2.3).

-- name: InsertMembership :exec
INSERT INTO organisation_memberships (org_id, user_id, role, invited_by, contact_email)
VALUES (@org_id, @user_id, @role, @invited_by, @contact_email);

-- name: ResolveMembership :one
-- Called with only mc.user_id set: the caller's own membership row is visible, and the
-- organisation row is visible only while that membership is active.
SELECT m.org_id, m.user_id, m.role, m.status, o.slug, o.name, o.status AS org_status
FROM organisation_memberships m
JOIN organisations o ON o.id = m.org_id
WHERE m.org_id = @org_id AND m.user_id = @user_id;

-- name: ListMyOrganisations :many
SELECT o.id, o.slug, o.name, o.status, m.role, m.joined_at
FROM organisation_memberships m
JOIN organisations o ON o.id = m.org_id
WHERE m.user_id = @user_id AND m.status = 'active'
ORDER BY lower(o.name), o.id;

-- name: ListMembers :many
-- Keyset pagination on (lower(display_name), user_id); sort_name is returned so the cursor uses
-- exactly the database's own lower-casing.
SELECT m.user_id, u.display_name, lower(u.display_name)::text AS sort_name, m.role, m.status, m.title,
       m.contact_email, m.joined_at, m.status_changed_at
FROM organisation_memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = @org_id
  AND (sqlc.narg(role)::text IS NULL OR m.role = sqlc.narg(role)::text)
  AND (sqlc.narg(status)::text IS NULL OR m.status = sqlc.narg(status)::text)
  AND (sqlc.narg(query)::text IS NULL OR u.display_name ILIKE '%' || sqlc.narg(query)::text || '%'
       OR m.contact_email ILIKE '%' || sqlc.narg(query)::text || '%')
  AND (sqlc.narg(after_name)::text IS NULL
       OR (lower(u.display_name), m.user_id) > (sqlc.narg(after_name)::text, sqlc.narg(after_id)::uuid))
ORDER BY lower(u.display_name), m.user_id
LIMIT @row_limit;

-- name: GetMember :one
SELECT m.user_id, u.display_name, m.role, m.status, m.title, m.contact_email, m.joined_at, m.status_changed_at
FROM organisation_memberships m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = @org_id AND m.user_id = @user_id;

-- name: GetMemberForUpdate :one
SELECT user_id, role, status FROM organisation_memberships
WHERE org_id = @org_id AND user_id = @user_id
FOR UPDATE;

-- name: UpdateMemberRole :exec
UPDATE organisation_memberships SET role = @role
WHERE org_id = @org_id AND user_id = @user_id;

-- name: UpdateMemberStatus :exec
UPDATE organisation_memberships SET status = @status, status_changed_at = now()
WHERE org_id = @org_id AND user_id = @user_id;

-- name: EmailIsMember :one
SELECT mc_org_email_is_member(@org_id::uuid, @email::text)::boolean AS is_member;
