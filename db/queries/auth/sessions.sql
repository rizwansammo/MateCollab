-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, amr, idle_expires_at, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, created_at, reauthenticated_at;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now(), idle_expires_at = $2
WHERE id = $1 AND revoked_at IS NULL;

-- name: ListActiveSessions :many
SELECT id, amr, created_at, last_seen_at, idle_expires_at, expires_at, ip, user_agent
FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now() AND idle_expires_at > now()
ORDER BY last_seen_at DESC;

-- name: RevokeSession :one
UPDATE sessions SET revoked_at = now(), revoked_reason = $3
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
RETURNING token_hash;

-- name: RevokeOtherSessions :many
UPDATE sessions SET revoked_at = now(), revoked_reason = $3
WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL
RETURNING token_hash;

-- name: RevokeAllSessions :many
UPDATE sessions SET revoked_at = now(), revoked_reason = $2
WHERE user_id = $1 AND revoked_at IS NULL
RETURNING token_hash;

-- name: MarkReauthenticated :exec
-- The application clock is used so step-up checks compare like with like.
UPDATE sessions SET reauthenticated_at = @reauthenticated_at WHERE id = @id AND revoked_at IS NULL;
