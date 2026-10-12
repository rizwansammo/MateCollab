-- name: GetTOTPFactor :one
SELECT user_id, secret_ciphertext, key_id, confirmed_at, last_used_step, created_at
FROM user_totp_factors WHERE user_id = $1;

-- name: GetTOTPFactorForUpdate :one
SELECT user_id, secret_ciphertext, key_id, confirmed_at, last_used_step, created_at
FROM user_totp_factors WHERE user_id = $1 FOR UPDATE;

-- name: SavePendingTOTP :exec
-- Enrolment starts (or restarts) unconfirmed; a confirmed factor is never overwritten.
INSERT INTO user_totp_factors (user_id, secret_ciphertext, key_id)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
  SET secret_ciphertext = EXCLUDED.secret_ciphertext, key_id = EXCLUDED.key_id,
      confirmed_at = NULL, last_used_step = NULL, created_at = now()
  WHERE user_totp_factors.confirmed_at IS NULL;

-- name: ConfirmTOTP :exec
UPDATE user_totp_factors SET confirmed_at = now(), last_used_step = $2
WHERE user_id = $1 AND confirmed_at IS NULL;

-- name: AdvanceTOTPStep :execrows
-- Replay protection: a step is accepted only once, and never one older than the last used.
UPDATE user_totp_factors SET last_used_step = sqlc.arg(step)::bigint
WHERE user_id = sqlc.arg(user_id) AND (last_used_step IS NULL OR last_used_step < sqlc.arg(step)::bigint);

-- name: DeleteTOTP :exec
DELETE FROM user_totp_factors WHERE user_id = $1;

-- name: DeleteRecoveryCodes :exec
DELETE FROM user_recovery_codes WHERE user_id = $1;

-- name: InsertRecoveryCode :exec
INSERT INTO user_recovery_codes (user_id, code_hash, key_id) VALUES ($1, $2, $3);

-- name: ListUnusedRecoveryCodesForUpdate :many
SELECT id, code_hash, key_id FROM user_recovery_codes
WHERE user_id = $1 AND used_at IS NULL
FOR UPDATE;

-- name: UseRecoveryCode :exec
UPDATE user_recovery_codes SET used_at = now() WHERE id = $1 AND used_at IS NULL;

-- name: CountUnusedRecoveryCodes :one
SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL;
