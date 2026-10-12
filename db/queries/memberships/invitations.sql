-- Invitations. Only SHA-256 token hashes are stored; the token is rotated on resend.

-- name: CreateInvitation :one
INSERT INTO invitations (org_id, email, role, token_hash, invited_by, expires_at)
VALUES (@org_id, @email, @role, @token_hash, @invited_by, @expires_at)
RETURNING id, org_id, email, role, invited_by, expires_at, last_sent_at, send_count,
          accepted_at, accepted_by, revoked_at, created_at;

-- name: RevokeLapsedInvitations :exec
-- An expired, unanswered invitation for the same address is revoked before re-inviting, so the
-- pending-invitation index only ever guards live invitations.
UPDATE invitations SET revoked_at = now()
WHERE org_id = @org_id AND lower(email) = lower(@email)
  AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at <= now();

-- name: ListInvitations :many
SELECT i.id, i.org_id, i.email, i.role, i.invited_by, i.expires_at, i.last_sent_at, i.send_count,
       i.accepted_at, i.accepted_by, i.revoked_at, i.created_at, u.display_name AS inviter_name
FROM invitations i
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.org_id = @org_id
  AND (NOT @pending_only::boolean
       OR (i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > now()))
  AND (sqlc.narg(before_created)::timestamptz IS NULL
       OR (i.created_at, i.id) < (sqlc.narg(before_created)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY i.created_at DESC, i.id DESC
LIMIT @row_limit;

-- name: GetInvitationForUpdate :one
SELECT id, org_id, email, role, invited_by, expires_at, last_sent_at, send_count,
       accepted_at, accepted_by, revoked_at, created_at
FROM invitations
WHERE org_id = @org_id AND id = @invitation_id
FOR UPDATE;

-- name: RevokeInvitation :exec
UPDATE invitations SET revoked_at = now()
WHERE org_id = @org_id AND id = @invitation_id AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: RotateInvitationToken :one
UPDATE invitations
SET token_hash = @token_hash, expires_at = @expires_at, last_sent_at = now(), send_count = send_count + 1
WHERE org_id = @org_id AND id = @invitation_id AND accepted_at IS NULL AND revoked_at IS NULL
RETURNING id, org_id, email, role, invited_by, expires_at, last_sent_at, send_count,
          accepted_at, accepted_by, revoked_at, created_at;

-- name: MarkInvitationAccepted :one
-- Single use: matches only a live invitation, and returns the stored role and inviter so the
-- membership is built from the invitation row itself.
UPDATE invitations SET accepted_at = now(), accepted_by = @accepted_by
WHERE org_id = @org_id AND id = @invitation_id
  AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()
RETURNING role, invited_by, email;
