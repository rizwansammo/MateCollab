-- Organisation audit log (append-only for the runtime role).

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (org_id, actor_type, actor_user_id, action, target_type, target_id, outcome,
                        ip, user_agent, session_id, request_id, metadata)
VALUES (@org_id, @actor_type, @actor_user_id, @action, @target_type, @target_id, @outcome,
        @ip, @user_agent, @session_id, @request_id, @metadata);

-- name: ListAuditLogs :many
-- Newest first, keyset-paginated on (created_at, id). `action` matches an exact action or a
-- prefix group ("member" matches "member.role_changed").
SELECT a.id, a.actor_type, a.actor_user_id, u.display_name AS actor_name, a.action, a.target_type,
       a.target_id, a.outcome, a.ip, a.user_agent, a.request_id, a.metadata, a.created_at
FROM audit_logs a
LEFT JOIN users u ON u.id = a.actor_user_id
WHERE a.org_id = @org_id
  AND (sqlc.narg(actor_user_id)::uuid IS NULL OR a.actor_user_id = sqlc.narg(actor_user_id)::uuid)
  AND (sqlc.narg(action)::text IS NULL OR a.action = sqlc.narg(action)::text
       OR starts_with(a.action, sqlc.narg(action)::text || '.'))
  AND (sqlc.narg(target_type)::text IS NULL OR a.target_type = sqlc.narg(target_type)::text)
  AND (sqlc.narg(target_id)::text IS NULL OR a.target_id = sqlc.narg(target_id)::text)
  AND (sqlc.narg(created_from)::timestamptz IS NULL OR a.created_at >= sqlc.narg(created_from)::timestamptz)
  AND (sqlc.narg(created_to)::timestamptz IS NULL OR a.created_at < sqlc.narg(created_to)::timestamptz)
  AND (sqlc.narg(before_created)::timestamptz IS NULL
       OR (a.created_at, a.id) < (sqlc.narg(before_created)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY a.created_at DESC, a.id DESC
LIMIT @row_limit;

-- name: CreateAuditExport :one
INSERT INTO audit_exports (org_id, requested_by, filters, expires_at)
VALUES (@org_id, @requested_by, @filters, @expires_at)
RETURNING id, status, filters, row_count, error, created_at, completed_at, expires_at;

-- name: GetAuditExport :one
SELECT id, status, filters, object_key, row_count, error, created_at, completed_at, expires_at
FROM audit_exports WHERE org_id = @org_id AND id = @export_id;

-- name: ListAuditExports :many
SELECT id, status, filters, row_count, error, created_at, completed_at, expires_at
FROM audit_exports WHERE org_id = @org_id
ORDER BY created_at DESC LIMIT 20;

-- name: ClaimAuditExport :one
UPDATE audit_exports SET status = 'running'
WHERE org_id = @org_id AND id = @export_id AND status IN ('pending', 'running')
RETURNING filters, expires_at;

-- name: CompleteAuditExport :exec
UPDATE audit_exports SET status = 'completed', object_key = @object_key, row_count = @row_count, completed_at = now()
WHERE org_id = @org_id AND id = @export_id;

-- name: FailAuditExport :exec
UPDATE audit_exports SET status = 'failed', error = @error, completed_at = now()
WHERE org_id = @org_id AND id = @export_id;

-- name: ExpireAuditExport :exec
UPDATE audit_exports SET status = 'expired', object_key = NULL
WHERE org_id = @org_id AND id = @export_id;
