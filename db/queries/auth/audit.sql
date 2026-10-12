-- Account-level security history (never passwords, codes or tokens in metadata).

-- name: InsertAccountEvent :exec
INSERT INTO account_audit_events (user_id, action, outcome, ip, user_agent, session_id, request_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListAccountEvents :many
SELECT id, action, outcome, ip, user_agent, created_at
FROM account_audit_events
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;
