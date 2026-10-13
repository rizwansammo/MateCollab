-- Users module: global profile, email addresses and preferences. Runs with mc.user_id set
-- (RLS: own rows; co-members may read profiles in an organisation context).

-- name: CreateUser :exec
INSERT INTO users (id, display_name, timezone, locale) VALUES ($1, $2, $3, $4);

-- name: GetUser :one
SELECT id, display_name, timezone, locale, status, avatar_version, created_at,
       deletion_requested_at, deletion_scheduled_at
FROM users WHERE id = $1;

-- name: ScheduleDeletion :execrows
UPDATE users
SET deletion_requested_at = @requested_at, deletion_scheduled_at = @scheduled_at, updated_at = now()
WHERE id = @id AND status = 'active' AND deletion_requested_at IS NULL;

-- name: CancelDeletion :execrows
UPDATE users
SET deletion_requested_at = NULL, deletion_scheduled_at = NULL, updated_at = now()
WHERE id = @id AND deletion_requested_at IS NOT NULL;

-- name: UpdateProfile :one
UPDATE users
SET display_name = $2, timezone = $3, locale = $4, updated_at = now()
WHERE id = $1
RETURNING id, display_name, timezone, locale, status, avatar_version, created_at;

-- name: CreateEmail :one
INSERT INTO user_emails (user_id, email, is_primary, verified_at)
VALUES ($1, $2, $3, $4)
RETURNING id, email, is_primary, verified_at, created_at;

-- name: ListEmails :many
SELECT id, email, is_primary, verified_at, created_at
FROM user_emails WHERE user_id = $1
ORDER BY is_primary DESC, created_at;

-- name: GetEmail :one
SELECT id, email, is_primary, verified_at, created_at
FROM user_emails WHERE id = $1 AND user_id = $2;

-- name: PrimaryEmail :one
SELECT id, email, is_primary, verified_at, created_at
FROM user_emails WHERE user_id = $1 AND is_primary;

-- name: MarkEmailVerified :execrows
UPDATE user_emails SET verified_at = now()
WHERE id = $1 AND user_id = $2 AND verified_at IS NULL;

-- name: DeleteEmail :execrows
DELETE FROM user_emails WHERE id = $1 AND user_id = $2 AND NOT is_primary;

-- name: ClearPrimaryEmail :exec
UPDATE user_emails SET is_primary = false WHERE user_id = $1 AND is_primary;

-- name: SetPrimaryEmail :execrows
UPDATE user_emails SET is_primary = true
WHERE id = $1 AND user_id = $2 AND verified_at IS NOT NULL;

-- name: GetPreferences :one
SELECT prefs, schema_version, updated_at FROM user_preferences WHERE user_id = $1;

-- name: SavePreferences :exec
INSERT INTO user_preferences (user_id, prefs, schema_version, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (user_id) DO UPDATE
  SET prefs = EXCLUDED.prefs, schema_version = EXCLUDED.schema_version, updated_at = now();
