package store

// Hand-written because sqlc cannot type table-returning functions (see the auth store).

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const invitationByToken = `SELECT id, org_id, email, role, expires_at, accepted_at, revoked_at,
       org_name, org_slug, org_status, inviter_name
FROM mc_invitation_by_token($1)`

// InvitationByTokenRow is an invitation with the organisation and inviter shown to the invitee.
type InvitationByTokenRow struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Email       string
	Role        string
	ExpiresAt   time.Time
	AcceptedAt  *time.Time
	RevokedAt   *time.Time
	OrgName     string
	OrgSlug     string
	OrgStatus   string
	InviterName *string
}

// InvitationByToken resolves an invitation before the invitee has a membership.
func (q *Queries) InvitationByToken(ctx context.Context, tokenHash []byte) (InvitationByTokenRow, error) {
	var r InvitationByTokenRow
	err := q.db.QueryRow(ctx, invitationByToken, tokenHash).Scan(&r.ID, &r.OrgID, &r.Email, &r.Role, &r.ExpiresAt,
		&r.AcceptedAt, &r.RevokedAt, &r.OrgName, &r.OrgSlug, &r.OrgStatus, &r.InviterName)
	return r, err
}
