-- Single-use, short-lived tokens. Only SHA-256 hashes are stored.

-- name: CreateAuthToken :one
INSERT INTO auth_tokens (user_id, purpose, token_hash, data, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: GetAuthTokenForUpdate :one
SELECT id, user_id, purpose, data, attempts, expires_at, used_at
FROM auth_tokens
WHERE token_hash = $1 AND purpose = $2
FOR UPDATE;

-- name: UseAuthToken :exec
UPDATE auth_tokens SET used_at = now() WHERE id = $1;

-- name: IncrementAuthTokenAttempts :one
UPDATE auth_tokens SET attempts = attempts + 1 WHERE id = $1 RETURNING attempts;

-- name: InvalidateUserTokens :exec
-- A newly issued token supersedes earlier unused ones of the same purpose.
UPDATE auth_tokens SET used_at = now()
WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL;
