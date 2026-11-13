// Package memberships owns organisation membership: roles and status, the member directory,
// the user's organisation list and invitations (BACKEND.md §2). Every permission decision goes
// through internal/authz; every tenant query runs in a tenant transaction and also names the
// organisation explicitly.
package memberships

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/memberships/store"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/cursor"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/users"
)

// InvitationTTL is how long an invitation link stays valid.
const InvitationTTL = 7 * 24 * time.Hour

var (
	limitInvitesPerInviter = ratelimit.PerHour(30)
	limitInvitesPerOrg     = ratelimit.PerDay(200)
	limitAcceptIP          = ratelimit.PerHour(30)
)

// Deps are the collaborators of the memberships service.
type Deps struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Auth      *auth.Service
	Users     *users.Service
	Outbox    *emails.Outbox
	Limiter   *ratelimit.Limiter
	CursorKey []byte
	PublicURL *url.URL
	Logger    *slog.Logger
	Now       func() time.Time
}

// Service implements membership and invitation use cases.
type Service struct{ Deps }

// New returns a memberships service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{Deps: d}
}

// ---- resolution (organisation context) -------------------------------------------------------

// Resolved is the caller's membership in one organisation.
type Resolved struct {
	OrgID     uuid.UUID   `json:"org_id"`
	Slug      string      `json:"slug"`
	Name      string      `json:"name"`
	OrgStatus string      `json:"org_status"`
	Role      tenant.Role `json:"role"`
	Status    string      `json:"status"`
}

const membershipCacheTTL = 60 * time.Second

func membershipKey(org tenant.OrgID, user tenant.UserID) string {
	return redisx.Key("org", org.String(), "member", user.String())
}

func orgEpochKey(org tenant.OrgID) string { return redisx.Key("org", org.String(), "epoch") }

// InvalidateOrganisation drops every cached membership resolution of org (after a rename or a
// status change), by moving its epoch: entries cached under an older epoch are ignored.
func (s *Service) InvalidateOrganisation(ctx context.Context, org tenant.OrgID) {
	_ = s.Redis.Incr(context.WithoutCancel(ctx), orgEpochKey(org)).Err()
}

// Resolve returns the user's membership in org. It is cached briefly and invalidated on every
// change; a missing membership is reported as not found (indistinguishable from a missing
// organisation).
func (s *Service) Resolve(ctx context.Context, user tenant.UserID, org tenant.OrgID) (Resolved, error) {
	key := membershipKey(org, user)
	epoch, _ := s.Redis.Get(ctx, orgEpochKey(org)).Int64()
	if raw, err := s.Redis.Get(ctx, key).Bytes(); err == nil {
		var r cachedResolution
		if json.Unmarshal(raw, &r) == nil && r.Epoch == epoch {
			return r.Resolved, nil
		}
	}
	var out Resolved
	err := db.InUserTx(ctx, s.Pool, user, func(tx *db.Tx) error {
		row, err := store.New(tx).ResolveMembership(ctx, store.ResolveMembershipParams{OrgID: uuid.UUID(org), UserID: uuid.UUID(user)})
		if err != nil {
			return err
		}
		out = Resolved{OrgID: row.OrgID, Slug: row.Slug, Name: row.Name, OrgStatus: row.OrgStatus, Role: tenant.Role(row.Role), Status: row.Status}
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Resolved{}, apperr.NotFound()
	}
	if err != nil {
		return Resolved{}, err
	}
	if raw, err := json.Marshal(cachedResolution{Resolved: out, Epoch: epoch}); err == nil {
		_ = s.Redis.Set(ctx, key, raw, membershipCacheTTL).Err()
	}
	return out, nil
}

type cachedResolution struct {
	Resolved
	Epoch int64 `json:"epoch"`
}

func (s *Service) invalidate(ctx context.Context, org tenant.OrgID, user tenant.UserID) {
	_ = s.Redis.Del(context.WithoutCancel(ctx), membershipKey(org, user)).Err()
}

// MyOrganisation is an entry in the organisation switcher.
type MyOrganisation struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Status   string
	Role     tenant.Role
	JoinedAt time.Time
}

// MyOrganisations lists the organisations where the user has an active membership.
func (s *Service) MyOrganisations(ctx context.Context, user tenant.UserID) ([]MyOrganisation, error) {
	var out []MyOrganisation
	err := db.InUserTx(ctx, s.Pool, user, func(tx *db.Tx) error {
		rows, err := store.New(tx).ListMyOrganisations(ctx, uuid.UUID(user))
		for _, r := range rows {
			out = append(out, MyOrganisation{ID: r.ID, Slug: r.Slug, Name: r.Name, Status: r.Status, Role: tenant.Role(r.Role), JoinedAt: r.JoinedAt})
		}
		return err
	})
	return out, err
}

// OwnedOrganisations names the organisations the user owns (blocks account deletion).
func (s *Service) OwnedOrganisations(ctx context.Context, user tenant.UserID) ([]string, error) {
	list, err := s.MyOrganisations(ctx, user)
	var owned []string
	for _, o := range list {
		if o.Role == tenant.RoleOwner {
			owned = append(owned, o.Name)
		}
	}
	return owned, err
}

// AddOwner creates the Owner membership of a new organisation (tx carries its tenant context).
func (s *Service) AddOwner(ctx context.Context, tx *db.Tx, org tenant.OrgID, user tenant.UserID, contactEmail string) error {
	return store.New(tx).InsertMembership(ctx, store.InsertMembershipParams{
		OrgID: uuid.UUID(org), UserID: uuid.UUID(user), Role: string(tenant.RoleOwner), ContactEmail: &contactEmail,
	})
}

// ---- members ------------------------------------------------------------------------------------

// Member is a directory entry.
type Member struct {
	UserID          uuid.UUID
	DisplayName     string
	Role            tenant.Role
	Status          string
	Title           *string
	Email           *string
	JoinedAt        time.Time
	StatusChangedAt *time.Time
}

// MemberFilter narrows the directory.
type MemberFilter struct {
	Role   string
	Status string
	Query  string
	Cursor string
	Limit  int
}

type memberCursor struct {
	Name string    `json:"n"`
	ID   uuid.UUID `json:"i"`
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ListMembers returns one page of the directory and the next cursor ("" on the last page).
func (s *Service) ListMembers(ctx context.Context, a tenant.Actor, f MemberFilter) ([]Member, string, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgDirectoryView); err != nil {
		return nil, "", err
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	scope := fmt.Sprintf("members:%s:%q:%q:%q", a.OrgID, f.Role, f.Status, f.Query)
	var after *memberCursor
	if f.Cursor != "" {
		var c memberCursor
		if err := cursor.Decode(s.CursorKey, scope, f.Cursor, &c); err != nil {
			return nil, "", err
		}
		after = &c
	}
	var out []Member
	next := ""
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		params := store.ListMembersParams{
			OrgID: uuid.UUID(a.OrgID), Role: nilIfEmpty(f.Role), Status: nilIfEmpty(f.Status), Query: nilIfEmpty(f.Query),
			RowLimit: int32(f.Limit + 1),
		}
		if after != nil {
			params.AfterName, params.AfterID = &after.Name, &after.ID
		}
		rows, err := store.New(tx).ListMembers(ctx, params)
		if err != nil {
			return err
		}
		for i, r := range rows {
			if i == f.Limit {
				last := rows[i-1]
				c, err := cursor.Encode(s.CursorKey, scope, memberCursor{Name: last.SortName, ID: last.UserID})
				if err != nil {
					return err
				}
				next = c
				break
			}
			out = append(out, Member{UserID: r.UserID, DisplayName: r.DisplayName, Role: tenant.Role(r.Role), Status: r.Status,
				Title: r.Title, Email: r.ContactEmail, JoinedAt: r.JoinedAt, StatusChangedAt: r.StatusChangedAt})
		}
		return nil
	})
	return out, next, err
}

// GetMember returns one member. Every member can see themselves; seeing others needs the
// directory permission.
func (s *Service) GetMember(ctx context.Context, a tenant.Actor, user uuid.UUID) (Member, error) {
	if tenant.UserID(user) != a.UserID {
		if err := authz.Check(a, authz.SettingsFrom(ctx), authz.OrgDirectoryView); err != nil {
			return Member{}, err
		}
	}
	var out Member
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		out, err = s.getMember(ctx, tx, a.OrgID, user)
		return err
	})
	return out, err
}

func (s *Service) getMember(ctx context.Context, tx *db.Tx, org tenant.OrgID, user uuid.UUID) (Member, error) {
	r, err := store.New(tx).GetMember(ctx, store.GetMemberParams{OrgID: uuid.UUID(org), UserID: user})
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, apperr.NotFound()
	}
	if err != nil {
		return Member{}, err
	}
	return Member{UserID: r.UserID, DisplayName: r.DisplayName, Role: tenant.Role(r.Role), Status: r.Status,
		Title: r.Title, Email: r.ContactEmail, JoinedAt: r.JoinedAt, StatusChangedAt: r.StatusChangedAt}, nil
}

// MemberUpdate changes another member's role and/or status.
type MemberUpdate struct {
	Role   *string
	Status *string
}

// UpdateMember applies a role or status change under the member-management rules.
func (s *Service) UpdateMember(ctx context.Context, a tenant.Actor, user uuid.UUID, in MemberUpdate, meta httpx.ClientMeta) (Member, error) {
	var fields []apperr.FieldError
	if in.Role == nil && in.Status == nil {
		fields = append(fields, apperr.Field("role", "required", "Change the role or the status."))
	}
	if in.Role != nil && !tenant.Role(*in.Role).Valid() {
		fields = append(fields, apperr.Field("role", "invalid", "Choose Owner, Admin, Member or Guest."))
	}
	if in.Status != nil && *in.Status != "active" && *in.Status != "suspended" && *in.Status != "deactivated" {
		fields = append(fields, apperr.Field("status", "invalid", "Choose active, suspended or deactivated."))
	}
	if len(fields) > 0 {
		return Member{}, apperr.Validation(fields...)
	}
	var out Member
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		q := store.New(tx)
		cur, err := q.GetMemberForUpdate(ctx, store.GetMemberForUpdateParams{OrgID: uuid.UUID(a.OrgID), UserID: user})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		change := authz.MemberChange{TargetUserID: tenant.UserID(user), TargetRole: tenant.Role(cur.Role)}
		if in.Role != nil && *in.Role != cur.Role {
			change.NewRole = tenant.Role(*in.Role)
		}
		if in.Status != nil && *in.Status != cur.Status {
			change.NewStatus = *in.Status
		}
		if err := authz.CheckMemberChange(a, authz.SettingsFrom(ctx), change); err != nil {
			return err
		}
		if change.NewRole != "" {
			if err := q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{OrgID: uuid.UUID(a.OrgID), UserID: user, Role: string(change.NewRole)}); err != nil {
				return err
			}
			// A Member becoming a Guest must not retain broad existing Space access.
			// Grant only via explicit Space membership operations after this change.
			if change.NewRole == tenant.RoleGuest {
				if _, err := tx.Exec(ctx, `DELETE FROM space_members WHERE org_id=$1 AND user_id=$2`, uuid.UUID(a.OrgID), user); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE conversation_members cm SET left_at=now()
                    FROM conversations c WHERE cm.org_id=$1 AND cm.user_id=$2
                    AND c.org_id=cm.org_id AND c.id=cm.conversation_id AND c.kind='channel'
                    AND cm.left_at IS NULL`, uuid.UUID(a.OrgID), user); err != nil {
					return err
				}
			}
			if err := audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "member.role_changed", TargetType: "member", TargetID: user.String(),
				Metadata: map[string]any{"from": cur.Role, "to": string(change.NewRole)}}); err != nil {
				return err
			}
		}
		if change.NewStatus != "" {
			if err := q.UpdateMemberStatus(ctx, store.UpdateMemberStatusParams{OrgID: uuid.UUID(a.OrgID), UserID: user, Status: change.NewStatus}); err != nil {
				return err
			}
			if err := audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "member.status_changed", TargetType: "member", TargetID: user.String(),
				Metadata: map[string]any{"from": cur.Status, "to": change.NewStatus}}); err != nil {
				return err
			}
		}
		out, err = s.getMember(ctx, tx, a.OrgID, user)
		return err
	})
	if err == nil {
		s.invalidate(ctx, a.OrgID, tenant.UserID(user)) // access changes take effect immediately
	}
	return out, err
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
