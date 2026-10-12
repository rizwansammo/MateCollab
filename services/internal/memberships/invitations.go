package memberships

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/memberships/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/cursor"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/users"
)

// Invitation is the admin view of an invitation (never includes the token).
type Invitation struct {
	ID          uuid.UUID
	Email       string
	Role        tenant.Role
	Status      string // pending | accepted | revoked | expired
	InvitedBy   uuid.UUID
	InviterName string
	ExpiresAt   time.Time
	LastSentAt  time.Time
	SendCount   int32
	CreatedAt   time.Time
	AcceptedAt  *time.Time
}

func invitationStatus(now time.Time, accepted, revoked *time.Time, expires time.Time) string {
	switch {
	case accepted != nil:
		return "accepted"
	case revoked != nil:
		return "revoked"
	case !now.Before(expires):
		return "expired"
	}
	return "pending"
}

func roleLabel(r tenant.Role) string {
	switch r {
	case tenant.RoleAdmin:
		return "an Admin"
	case tenant.RoleGuest:
		return "a Guest"
	}
	return "a Member"
}

func (s *Service) invitationLink(token string) string {
	u := *s.PublicURL
	u.Path, u.RawQuery, u.Fragment = "/invite", "", "token="+token
	return u.String()
}

// Invite creates an invitation and emails the link. The role determines which permission is
// required; the organisation always comes from the actor.
func (s *Service) Invite(ctx context.Context, a tenant.Actor, email, role string, meta httpx.ClientMeta) (Invitation, error) {
	email = strings.TrimSpace(email)
	r := tenant.Role(role)
	var fields []apperr.FieldError
	if at := strings.LastIndex(email, "@"); len(email) < 3 || len(email) > 254 || at <= 0 || at == len(email)-1 || strings.ContainsAny(email, " \r\n\t<>") {
		fields = append(fields, apperr.Field("email", "invalid", "Enter a valid email address."))
	}
	perm, ok := authz.InvitePermissionFor(r)
	if !ok {
		fields = append(fields, apperr.Field("role", "invalid", "Choose Admin, Member or Guest."))
	}
	if len(fields) > 0 {
		return Invitation{}, apperr.Validation(fields...)
	}
	if err := authz.Check(a, authz.SettingsFrom(ctx), perm); err != nil {
		return Invitation{}, err
	}
	if err := s.Limiter.Check(ctx, "invitations.inviter", a.UserID.String(), limitInvitesPerInviter); err != nil {
		return Invitation{}, err
	}
	if err := s.Limiter.Check(ctx, "invitations.org", a.OrgID.String(), limitInvitesPerOrg); err != nil {
		return Invitation{}, err
	}
	org, err := s.Resolve(ctx, a.UserID, a.OrgID)
	if err != nil {
		return Invitation{}, err
	}
	var out Invitation
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		q := store.New(tx)
		member, err := q.EmailIsMember(ctx, store.EmailIsMemberParams{OrgID: uuid.UUID(a.OrgID), Email: email})
		if err != nil {
			return err
		}
		if member {
			return apperr.Conflict("already_member", "This person is already a member of the organisation.")
		}
		if err := q.RevokeLapsedInvitations(ctx, store.RevokeLapsedInvitationsParams{OrgID: uuid.UUID(a.OrgID), Email: email}); err != nil {
			return err
		}
		token, err := auth.NewToken()
		if err != nil {
			return err
		}
		row, err := q.CreateInvitation(ctx, store.CreateInvitationParams{
			OrgID: uuid.UUID(a.OrgID), Email: email, Role: role, TokenHash: auth.HashToken(token),
			InvitedBy: uuid.UUID(a.UserID), ExpiresAt: s.Now().Add(InvitationTTL),
		})
		if isUniqueViolation(err) {
			return apperr.Conflict("invitation_pending", "This address already has a pending invitation. Resend it instead.")
		}
		if err != nil {
			return err
		}
		inviter, _, err := s.Users.Profile(ctx, tx, a.UserID)
		if err != nil {
			return err
		}
		out = Invitation{ID: row.ID, Email: row.Email, Role: tenant.Role(row.Role), Status: "pending", InvitedBy: row.InvitedBy,
			InviterName: inviter, ExpiresAt: row.ExpiresAt, LastSentAt: row.LastSentAt, SendCount: row.SendCount, CreatedAt: row.CreatedAt}
		if err := audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "invitation.created", TargetType: "invitation",
			TargetID: row.ID.String(), Metadata: map[string]any{"role": role}}); err != nil {
			return err
		}
		return s.Outbox.Enqueue(ctx, tx, emails.Message{Template: emails.Invitation, To: email, Data: map[string]string{
			"inviter": inviter, "org": org.Name, "role": roleLabel(r), "link": s.invitationLink(token),
		}})
	})
	return out, err
}

type invitationCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

// ListInvitations returns one page of the organisation's invitations, newest first.
func (s *Service) ListInvitations(ctx context.Context, a tenant.Actor, pendingOnly bool, cur string, limit int) ([]Invitation, string, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgInvitationsView); err != nil {
		return nil, "", err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	scope := "invitations:" + a.OrgID.String()
	if pendingOnly {
		scope += ":pending"
	}
	var before *invitationCursor
	if cur != "" {
		var c invitationCursor
		if err := cursor.Decode(s.CursorKey, scope, cur, &c); err != nil {
			return nil, "", err
		}
		before = &c
	}
	var out []Invitation
	next := ""
	now := s.Now()
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		params := store.ListInvitationsParams{OrgID: uuid.UUID(a.OrgID), PendingOnly: pendingOnly, RowLimit: int32(limit + 1)}
		if before != nil {
			params.BeforeCreated, params.BeforeID = &before.CreatedAt, &before.ID
		}
		rows, err := store.New(tx).ListInvitations(ctx, params)
		if err != nil {
			return err
		}
		for i, r := range rows {
			if i == limit {
				last := rows[i-1]
				if next, err = cursor.Encode(s.CursorKey, scope, invitationCursor{CreatedAt: last.CreatedAt, ID: last.ID}); err != nil {
					return err
				}
				break
			}
			name := ""
			if r.InviterName != nil {
				name = *r.InviterName
			}
			out = append(out, Invitation{ID: r.ID, Email: r.Email, Role: tenant.Role(r.Role),
				Status: invitationStatus(now, r.AcceptedAt, r.RevokedAt, r.ExpiresAt), InvitedBy: r.InvitedBy, InviterName: name,
				ExpiresAt: r.ExpiresAt, LastSentAt: r.LastSentAt, SendCount: r.SendCount, CreatedAt: r.CreatedAt, AcceptedAt: r.AcceptedAt})
		}
		return nil
	})
	return out, next, err
}

// Resend issues a fresh link (rotating the token, so earlier links stop working) and extends
// the expiry. Expired invitations can be resent; accepted or revoked ones cannot.
func (s *Service) Resend(ctx context.Context, a tenant.Actor, id uuid.UUID, meta httpx.ClientMeta) (Invitation, error) {
	if err := s.Limiter.Check(ctx, "invitations.inviter", a.UserID.String(), limitInvitesPerInviter); err != nil {
		return Invitation{}, err
	}
	org, err := s.Resolve(ctx, a.UserID, a.OrgID)
	if err != nil {
		return Invitation{}, err
	}
	var out Invitation
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		q := store.New(tx)
		inv, err := q.GetInvitationForUpdate(ctx, store.GetInvitationForUpdateParams{OrgID: uuid.UUID(a.OrgID), InvitationID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if err := authz.CheckInvitationRevoke(a, authz.SettingsFrom(ctx), tenant.UserID(inv.InvitedBy), tenant.Role(inv.Role)); err != nil {
			return err
		}
		if inv.AcceptedAt != nil || inv.RevokedAt != nil {
			return apperr.Conflict("invitation_closed", "This invitation was already accepted or revoked.")
		}
		token, err := auth.NewToken()
		if err != nil {
			return err
		}
		row, err := q.RotateInvitationToken(ctx, store.RotateInvitationTokenParams{
			OrgID: uuid.UUID(a.OrgID), InvitationID: id, TokenHash: auth.HashToken(token), ExpiresAt: s.Now().Add(InvitationTTL),
		})
		if err != nil {
			return err
		}
		inviter, _, err := s.Users.Profile(ctx, tx, a.UserID)
		if err != nil {
			return err
		}
		out = Invitation{ID: row.ID, Email: row.Email, Role: tenant.Role(row.Role), Status: "pending", InvitedBy: row.InvitedBy,
			ExpiresAt: row.ExpiresAt, LastSentAt: row.LastSentAt, SendCount: row.SendCount, CreatedAt: row.CreatedAt}
		if err := audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "invitation.resent", TargetType: "invitation", TargetID: id.String()}); err != nil {
			return err
		}
		return s.Outbox.Enqueue(ctx, tx, emails.Message{Template: emails.Invitation, To: row.Email, Data: map[string]string{
			"inviter": inviter, "org": org.Name, "role": roleLabel(tenant.Role(row.Role)), "link": s.invitationLink(token),
		}})
	})
	return out, err
}

// Revoke cancels a pending invitation (idempotent for already revoked ones).
func (s *Service) Revoke(ctx context.Context, a tenant.Actor, id uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		q := store.New(tx)
		inv, err := q.GetInvitationForUpdate(ctx, store.GetInvitationForUpdateParams{OrgID: uuid.UUID(a.OrgID), InvitationID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if err := authz.CheckInvitationRevoke(a, authz.SettingsFrom(ctx), tenant.UserID(inv.InvitedBy), tenant.Role(inv.Role)); err != nil {
			return err
		}
		if inv.AcceptedAt != nil {
			return apperr.Conflict("invitation_used", "This invitation has already been accepted.")
		}
		if inv.RevokedAt != nil {
			return nil
		}
		if err := q.RevokeInvitation(ctx, store.RevokeInvitationParams{OrgID: uuid.UUID(a.OrgID), InvitationID: id}); err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "invitation.revoked", TargetType: "invitation", TargetID: id.String()})
	})
}

// ---- invitee side ----------------------------------------------------------------------------

// Preview is what the invitee sees before accepting.
type Preview struct {
	OrgID         uuid.UUID
	OrgName       string
	OrgSlug       string
	InviterName   string
	Email         string
	Role          tenant.Role
	ExpiresAt     time.Time
	Status        string
	AccountExists bool
}

var errInvalidInvitation = &apperr.Error{Kind: apperr.KindNotFound, Code: "invitation_invalid", Detail: "This invitation link is not valid. Ask for a new one."}

func (s *Service) lookup(ctx context.Context, token string) (store.InvitationByTokenRow, error) {
	if token == "" || len(token) > 128 {
		return store.InvitationByTokenRow{}, errInvalidInvitation
	}
	row, err := store.New(s.Pool).InvitationByToken(ctx, auth.HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errInvalidInvitation
	}
	return row, err
}

// PreviewInvitation describes an invitation to the holder of its link.
func (s *Service) PreviewInvitation(ctx context.Context, token string, meta httpx.ClientMeta) (Preview, error) {
	if err := s.Limiter.Check(ctx, "invitations.accept.ip", meta.IP.String(), limitAcceptIP); err != nil {
		return Preview{}, err
	}
	row, err := s.lookup(ctx, token)
	if err != nil {
		return Preview{}, err
	}
	if row.RevokedAt != nil || row.OrgStatus == "deleting" {
		return Preview{}, errInvalidInvitation
	}
	exists, err := s.Auth.AccountExists(ctx, row.Email)
	if err != nil {
		return Preview{}, err
	}
	p := Preview{OrgID: row.OrgID, OrgName: row.OrgName, OrgSlug: row.OrgSlug, Email: row.Email, Role: tenant.Role(row.Role),
		ExpiresAt: row.ExpiresAt, Status: invitationStatus(s.Now(), row.AcceptedAt, row.RevokedAt, row.ExpiresAt), AccountExists: exists}
	if row.InviterName != nil {
		p.InviterName = *row.InviterName
	}
	return p, nil
}

// AcceptInput is the invitee's input. Only name and password (for a new account) are taken
// from the client; the organisation and role always come from the stored invitation.
type AcceptInput struct {
	Token       string
	DisplayName string
	Password    string
}

// AcceptResult tells the client where it now belongs and, for a new account, its session.
type AcceptResult struct {
	OrgID   uuid.UUID
	OrgSlug string
	OrgName string
	Session *auth.NewSession
}

// Accept redeems an invitation. A signed-in user must own the invited address; an anonymous
// invitee either signs in first (an account exists) or creates an account here.
func (s *Service) Accept(ctx context.Context, in AcceptInput, principal *auth.Principal, meta httpx.ClientMeta) (AcceptResult, error) {
	if err := s.Limiter.Check(ctx, "invitations.accept.ip", meta.IP.String(), limitAcceptIP); err != nil {
		return AcceptResult{}, err
	}
	inv, err := s.lookup(ctx, in.Token)
	if err != nil {
		return AcceptResult{}, err
	}
	switch status := invitationStatus(s.Now(), inv.AcceptedAt, inv.RevokedAt, inv.ExpiresAt); {
	case status == "revoked" || inv.OrgStatus == "deleting":
		return AcceptResult{}, errInvalidInvitation
	case status == "accepted":
		return AcceptResult{}, apperr.Conflict("invitation_used", "This invitation has already been used.")
	case status == "expired":
		return AcceptResult{}, apperr.Conflict("invitation_expired", "This invitation has expired. Ask your organisation Admin for a new one.")
	}
	if inv.OrgStatus == "suspended" {
		return AcceptResult{}, apperr.Forbidden("org_suspended", "This organisation is suspended.")
	}
	org := tenant.OrgID(inv.OrgID)
	exists, err := s.Auth.AccountExists(ctx, inv.Email)
	if err != nil {
		return AcceptResult{}, err
	}

	var user tenant.UserID
	newAccount := false
	switch {
	case principal != nil:
		user = principal.UserID
		var owns bool
		if err := db.InUserTx(ctx, s.Pool, user, func(tx *db.Tx) error {
			owns, err = s.Users.HasVerifiedEmail(ctx, tx, user, inv.Email)
			return err
		}); err != nil {
			return AcceptResult{}, err
		}
		if !owns {
			return AcceptResult{}, apperr.Forbidden("invitation_email_mismatch",
				"This invitation was sent to a different email address. Sign in with that account to accept it.")
		}
	case exists:
		return AcceptResult{}, apperr.Unauthenticated("sign_in_required", "Sign in to accept this invitation.")
	default:
		newAccount = true
	}

	err = db.InTenantTx(ctx, s.Pool, tenant.Scope{OrgID: org, UserID: user}, func(tx *db.Tx) error {
		q := store.New(tx)
		if newAccount {
			created, _, err := s.Users.Create(ctx, tx, org, users.NewUser{DisplayName: in.DisplayName, Email: inv.Email, EmailVerified: true})
			if err != nil {
				return err
			}
			user = created
			if err := s.Auth.SetInitialPassword(ctx, tx, user, inv.Email, in.Password); err != nil {
				return err
			}
		}
		// The row update is the single-use gate: a concurrent or repeated accept matches nothing.
		accepted, err := q.MarkInvitationAccepted(ctx, store.MarkInvitationAcceptedParams{OrgID: inv.OrgID, InvitationID: inv.ID, AcceptedBy: ptr(uuid.UUID(user))})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("invitation_used", "This invitation has already been used.")
		}
		if err != nil {
			return err
		}
		existing, err := q.GetMemberForUpdate(ctx, store.GetMemberForUpdateParams{OrgID: inv.OrgID, UserID: uuid.UUID(user)})
		switch {
		case err == nil && existing.Status == "active":
			return apperr.Conflict("already_member", "You're already a member of this organisation.")
		case err == nil && existing.Role == string(tenant.RoleOwner):
			// The owner's membership is never rewritten by an invitation.
			return apperr.Conflict("already_member", "You're already a member of this organisation.")
		case err == nil:
			// A deactivated or suspended member was invited back: restore them with the invited role.
			if err := q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{OrgID: inv.OrgID, UserID: uuid.UUID(user), Role: accepted.Role}); err != nil {
				return err
			}
			if err := q.UpdateMemberStatus(ctx, store.UpdateMemberStatusParams{OrgID: inv.OrgID, UserID: uuid.UUID(user), Status: "active"}); err != nil {
				return err
			}
		case errors.Is(err, pgx.ErrNoRows):
			if err := q.InsertMembership(ctx, store.InsertMembershipParams{OrgID: inv.OrgID, UserID: uuid.UUID(user), Role: accepted.Role,
				InvitedBy: &accepted.InvitedBy, ContactEmail: &accepted.Email}); err != nil {
				return err
			}
		default:
			return err
		}
		return audit.Record(ctx, tx, org, user, sessionOf(principal), meta, audit.Entry{
			Action: "invitation.accepted", TargetType: "invitation", TargetID: inv.ID.String(),
			Metadata: map[string]any{"role": accepted.Role, "new_account": newAccount},
		})
	})
	if err != nil {
		return AcceptResult{}, err
	}
	s.invalidate(ctx, org, user)
	res := AcceptResult{OrgID: inv.OrgID, OrgSlug: inv.OrgSlug, OrgName: inv.OrgName}
	if newAccount {
		ns, err := s.Auth.StartSession(ctx, user, []string{"pwd"}, meta)
		if err != nil {
			return AcceptResult{}, err
		}
		res.Session = &ns
	}
	return res, nil
}

func sessionOf(p *auth.Principal) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return p.SessionID
}

func ptr[T any](v T) *T { return &v }
