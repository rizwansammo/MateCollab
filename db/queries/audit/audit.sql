-- Organisation audit log (append-only for the runtime role).

-- name: InsertAuditLog :exec
INSERT INTO audit_logs (org_id, actor_type, actor_user_id, action, target_type, target_id, outcome,
                        ip, user_agent, session_id, request_id, metadata)
VALUES (@org_id, @actor_type, @actor_user_id, @action, @target_type, @target_id, @outcome,
        @ip, @user_agent, @session_id, @request_id, @metadata);

-- name: ListAuditLogs :many
SELECT id, actor_type, actor_user_id, action, target_type, target_id, outcome, created_at, metadata
FROM audit_logs
WHERE org_id = @org_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;
