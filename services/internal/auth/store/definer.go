package store

// Hand-written because sqlc cannot type table-returning functions. These call the narrow
// SECURITY DEFINER lookups used before any user context exists (MULTITENANCY.md §2.5); the
// queries are fixed strings with bound parameters.

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const loginLookup = `SELECT user_id, user_status, display_name, password_hash, totp_enabled
FROM mc_auth_login_lookup($1)`

// LoginLookupRow is the minimal account data needed to check a password.
type LoginLookupRow struct {
	UserID       uuid.UUID
	UserStatus   string
	DisplayName  string
	PasswordHash *string
	TOTPEnabled  bool
}

// LoginLookup finds the account for a verified or primary email address.
func (q *Queries) LoginLookup(ctx context.Context, email string) (LoginLookupRow, error) {
	var r LoginLookupRow
	err := q.db.QueryRow(ctx, loginLookup, email).Scan(&r.UserID, &r.UserStatus, &r.DisplayName, &r.PasswordHash, &r.TOTPEnabled)
	return r, err
}

const lookupSession = `SELECT session_id, user_id, amr, created_at, last_seen_at, idle_expires_at, expires_at,
       reauthenticated_at, revoked_at, user_status, totp_enabled
FROM mc_session_lookup($1)`

// SessionLookupRow is a session joined with its user's status.
type SessionLookupRow struct {
	SessionID         uuid.UUID
	UserID            uuid.UUID
	Amr               []string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	ExpiresAt         time.Time
	ReauthenticatedAt time.Time
	RevokedAt         *time.Time
	UserStatus        string
	TOTPEnabled       bool
}

// LookupSession resolves a session by the SHA-256 of its token.
func (q *Queries) LookupSession(ctx context.Context, tokenHash []byte) (SessionLookupRow, error) {
	var r SessionLookupRow
	err := q.db.QueryRow(ctx, lookupSession, tokenHash).Scan(&r.SessionID, &r.UserID, &r.Amr, &r.CreatedAt,
		&r.LastSeenAt, &r.IdleExpiresAt, &r.ExpiresAt, &r.ReauthenticatedAt, &r.RevokedAt, &r.UserStatus, &r.TOTPEnabled)
	return r, err
}
