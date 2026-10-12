-- Identity tables owned by the auth module (RLS-exempt; MULTITENANCY.md §2.5). The
-- SECURITY DEFINER lookups (login, session) are in internal/auth/store/definer.go because sqlc
-- cannot type table-returning functions.

-- name: GetPasswordHash :one
SELECT password_hash FROM user_credentials WHERE user_id = $1;

-- name: SetPassword :exec
INSERT INTO user_credentials (user_id, password_hash, password_changed_at)
VALUES ($1, $2, now())
ON CONFLICT (user_id) DO UPDATE
  SET password_hash = EXCLUDED.password_hash, password_changed_at = now();

-- name: UpgradePasswordHash :exec
-- Rehash with current parameters after a successful login (password unchanged).
UPDATE user_credentials SET password_hash = $2 WHERE user_id = $1;
