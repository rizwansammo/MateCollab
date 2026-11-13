// Package collaboration implements Phase 3 Spaces and Channels on unified conversations.
// Every operation is tenant scoped and uses the central authz policy.
package collaboration

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/platform/textutil"
)

// Service maintains Space and Channel resources using the existing tenant transaction boundary.
type Service struct{ Pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{Pool: pool} }

type Space struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Overview    *string    `json:"overview"`
	Kind        string     `json:"kind"`
	Visibility  string     `json:"visibility"`
	CreatedBy   uuid.UUID  `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
	ViewerRole  string     `json:"viewer_role,omitempty"`
	Joined      bool       `json:"joined"`
}

type SpaceInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Overview    *string `json:"overview"`
	Kind        string  `json:"kind"`
	Visibility  string  `json:"visibility"`
}

type SpaceMember struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

var channelName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,78}[a-z0-9])?$`)

func textValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func optionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func unique(err error) bool { var p *pgconn.PgError; return errors.As(err, &p) && p.Code == "23505" }
func missing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound()
	}
	return err
}
func conflictName() error { return apperr.Conflict("name_taken", "This name is already in use.") }

func validateSpace(in SpaceInput) (SpaceInput, error) {
	in.Name = textutil.CleanName(in.Name)
	if n := textutil.Length(in.Name); n < 1 || n > 80 {
		return in, apperr.Validation(apperr.Field("name", "invalid", "Use a name of 1 to 80 characters."))
	}
	if in.Kind == "" {
		in.Kind = "team"
	}
	if in.Visibility == "" {
		in.Visibility = "public"
	}
	if in.Kind != "team" && in.Kind != "project" {
		return in, apperr.Validation(apperr.Field("kind", "invalid", "Choose Team or Project."))
	}
	if in.Visibility != "public" && in.Visibility != "private" {
		return in, apperr.Validation(apperr.Field("visibility", "invalid", "Choose Public or Private."))
	}
	for _, field := range []struct {
		name  string
		value *string
		max   int
	}{
		{"description", in.Description, 500}, {"overview", in.Overview, 5000},
	} {
		if field.value != nil && textutil.Length(*field.value) > field.max {
			return in, apperr.Validation(apperr.Field(field.name, "too_long", "The text exceeds the allowed length."))
		}
	}
	return in, nil
}

func spaceFacts(s Space) authz.SpaceFacts {
	return authz.SpaceFacts{Public: s.Visibility == "public", Joined: s.Joined, Admin: s.ViewerRole == "space_admin", Archived: s.ArchivedAt != nil}
}

const selectSpace = `
SELECT s.id,s.org_id,s.name,s.description,s.overview,s.kind,s.visibility,s.created_by,
       s.created_at,s.updated_at,s.archived_at,COALESCE(sm.role,'')
FROM spaces s LEFT JOIN space_members sm ON sm.org_id=s.org_id AND sm.space_id=s.id AND sm.user_id=$3
WHERE s.org_id=$1 AND s.id=$2`

func scanSpace(row pgx.Row) (Space, error) {
	var s Space
	err := row.Scan(&s.ID, &s.OrgID, &s.Name, &s.Description, &s.Overview, &s.Kind, &s.Visibility,
		&s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &s.ArchivedAt, &s.ViewerRole)
	s.Joined = s.ViewerRole != ""
	return s, err
}

func (s *Service) space(ctx context.Context, tx *db.Tx, a tenant.Actor, id uuid.UUID) (Space, error) {
	out, err := scanSpace(tx.QueryRow(ctx, selectSpace, uuid.UUID(a.OrgID), id, uuid.UUID(a.UserID)))
	return out, missing(err)
}

func (s *Service) Space(ctx context.Context, a tenant.Actor, id uuid.UUID) (Space, error) {
	var out Space
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		out, err = s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		return authz.CheckSpace(a, spaceFacts(out), authz.SpaceDiscover)
	})
	return out, err
}

// ListSpaces shows joined Spaces and discoverable public Spaces. Private names cannot leak.
func (s *Service) ListSpaces(ctx context.Context, a tenant.Actor) ([]Space, error) {
	out := []Space{}
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		rows, err := tx.Query(ctx, `SELECT s.id,s.org_id,s.name,s.description,s.overview,s.kind,s.visibility,s.created_by,
            s.created_at,s.updated_at,s.archived_at,COALESCE(sm.role,'')
            FROM spaces s LEFT JOIN space_members sm ON sm.org_id=s.org_id AND sm.space_id=s.id AND sm.user_id=$2
            WHERE s.org_id=$1 AND s.archived_at IS NULL
              AND (sm.user_id IS NOT NULL OR ($3 AND s.visibility IN ('public','private'))
                   OR (s.visibility='public' AND NOT $4))
            ORDER BY lower(s.name),s.id LIMIT 201`, uuid.UUID(a.OrgID), uuid.UUID(a.UserID),
			a.Role == tenant.RoleOwner || a.Role == tenant.RoleAdmin, a.Role == tenant.RoleGuest)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			item, err := scanSpace(rows)
			if err != nil {
				return err
			}
			if authz.CanDiscoverSpace(a, spaceFacts(item)) {
				out = append(out, item)
			}
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) creationPolicy(ctx context.Context, tx *db.Tx, a tenant.Actor) (authz.Settings, error) {
	settings := authz.DefaultSettings()
	var public, private *string
	err := tx.QueryRow(ctx, `SELECT settings->>'public_space_creation',settings->>'private_space_creation'
        FROM organisation_settings WHERE org_id=$1 AND section='messaging'`, uuid.UUID(a.OrgID)).Scan(&public, &private)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return settings, err
	}
	if public != nil {
		settings.PublicSpaceCreation = *public
	}
	if private != nil {
		settings.PrivateSpaceCreation = *private
	}
	return settings, nil
}

// CreateSpace atomically creates a Space, creator membership and the default #general Channel.
func (s *Service) CreateSpace(ctx context.Context, a tenant.Actor, in SpaceInput, meta httpx.ClientMeta) (Space, error) {
	in, err := validateSpace(in)
	if err != nil {
		return Space{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Space{}, err
	}
	channelID, err := uuid.NewV7()
	if err != nil {
		return Space{}, err
	}
	var out Space
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		settings, err := s.creationPolicy(ctx, tx, a)
		if err != nil {
			return err
		}
		if !authz.CanCreateSpace(a, settings, in.Visibility == "private") {
			return authz.ErrForbidden("forbidden")
		}
		err = tx.QueryRow(ctx, `INSERT INTO spaces (id,org_id,name,description,overview,kind,visibility,created_by)
            VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at,updated_at`,
			id, uuid.UUID(a.OrgID), in.Name, in.Description, in.Overview, in.Kind, in.Visibility, uuid.UUID(a.UserID)).
			Scan(&out.CreatedAt, &out.UpdatedAt)
		if unique(err) {
			return conflictName()
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO space_members (org_id,space_id,user_id,role,added_by)
                VALUES ($1,$2,$3,'space_admin',$3)`,
			uuid.UUID(a.OrgID), id, uuid.UUID(a.UserID)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO conversations (id,org_id,kind,space_id,name,visibility,is_default,created_by)
                VALUES ($1,$2,'channel',$3,'general','public',true,$4)`,
			channelID, uuid.UUID(a.OrgID), id, uuid.UUID(a.UserID)); err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.created", TargetType: "space",
			TargetID: id.String(), Metadata: map[string]any{"kind": in.Kind, "visibility": in.Visibility}})
	})
	out.ID = id
	out.OrgID = uuid.UUID(a.OrgID)
	out.Name = in.Name
	out.Description = in.Description
	out.Overview = in.Overview
	out.Kind = in.Kind
	out.Visibility = in.Visibility
	out.CreatedBy = uuid.UUID(a.UserID)
	out.ViewerRole = "space_admin"
	out.Joined = true
	return out, err
}

// UpdateSpace uses If-Match to prevent stale clients overwriting another administrator.
func (s *Service) UpdateSpace(ctx context.Context, a tenant.Actor, id uuid.UUID, in SpaceInput, expected time.Time, meta httpx.ClientMeta) (Space, error) {
	in, err := validateSpace(in)
	if err != nil {
		return Space{}, err
	}
	var out Space
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		before, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if err := authz.CheckSpace(a, spaceFacts(before), authz.SpaceUpdate); err != nil {
			return err
		}
		if before.UpdatedAt.UnixMicro() != expected.UnixMicro() {
			return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "This Space changed. Reload and try again."}
		}
		tag, err := tx.Exec(ctx, `UPDATE spaces SET name=$3,description=$4,overview=$5,kind=$6,visibility=$7,updated_at=now()
            WHERE org_id=$1 AND id=$2 AND archived_at IS NULL AND updated_at=$8`, uuid.UUID(a.OrgID), id,
			in.Name, in.Description, in.Overview, in.Kind, in.Visibility, expected)
		if unique(err) {
			return conflictName()
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "This Space changed. Reload and try again."}
		}
		out, err = s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.updated", TargetType: "space", TargetID: id.String()})
	})
	return out, err
}

func (s *Service) ArchiveSpace(ctx context.Context, a tenant.Actor, id uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM spaces WHERE org_id=$1 AND id=$2 FOR UPDATE`, uuid.UUID(a.OrgID), id); err != nil {
			return err
		}
		before, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if err := authz.CheckSpace(a, spaceFacts(before), authz.SpaceArchive); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE spaces SET archived_at=now(),updated_at=now()
            WHERE org_id=$1 AND id=$2`, uuid.UUID(a.OrgID), id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE conversations SET archived_at=now(),updated_at=now()
            WHERE org_id=$1 AND space_id=$2 AND kind='channel' AND archived_at IS NULL`, uuid.UUID(a.OrgID), id); err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.archived", TargetType: "space", TargetID: id.String()})
	})
}

func (s *Service) JoinSpace(ctx context.Context, a tenant.Actor, id uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if sp.Joined {
			return nil
		}
		if err := authz.CheckSpace(a, spaceFacts(sp), authz.SpaceJoin); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO space_members (org_id,space_id,user_id,role,added_by)
            VALUES ($1,$2,$3,'member',$3) ON CONFLICT DO NOTHING`,
			uuid.UUID(a.OrgID), id, uuid.UUID(a.UserID))
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.joined", TargetType: "space", TargetID: id.String()})
	})
}

func (s *Service) ListSpaceMembers(ctx context.Context, a tenant.Actor, id uuid.UUID) ([]SpaceMember, error) {
	out := []SpaceMember{}
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceRead); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT sm.user_id,u.display_name,sm.role,sm.joined_at
            FROM space_members sm JOIN users u ON u.id=sm.user_id
            JOIN organisation_memberships m ON m.org_id=sm.org_id AND m.user_id=sm.user_id AND m.status='active'
            WHERE sm.org_id=$1 AND sm.space_id=$2 ORDER BY lower(u.display_name),sm.user_id LIMIT 201`, uuid.UUID(a.OrgID), id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v SpaceMember
			if err := rows.Scan(&v.UserID, &v.DisplayName, &v.Role, &v.JoinedAt); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

// SetSpaceMember is available to Space Admins and organisation administrators.
// It cannot elevate an organisation role, or grant access outside this tenant.
func (s *Service) SetSpaceMember(ctx context.Context, a tenant.Actor, id, user uuid.UUID, role string, meta httpx.ClientMeta) error {
	if role != "space_admin" && role != "member" {
		return apperr.Validation(apperr.Field("role", "invalid", "Choose Space Admin or Member."))
	}
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT id FROM spaces WHERE org_id=$1 AND id=$2 FOR UPDATE`, uuid.UUID(a.OrgID), id); err != nil {
			return err
		}
		sp, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceMembersManage); err != nil {
			return err
		}
		var targetRole string
		if err = tx.QueryRow(ctx, `SELECT role FROM organisation_memberships
            WHERE org_id=$1 AND user_id=$2 AND status='active'`, uuid.UUID(a.OrgID), user).Scan(&targetRole); err != nil {
			return missing(err)
		}
		if role == "space_admin" && targetRole == "guest" {
			return authz.ErrForbidden("guest_cannot_manage_space")
		}
		var previousRole string
		oldErr := tx.QueryRow(ctx, `SELECT role FROM space_members WHERE org_id=$1 AND space_id=$2 AND user_id=$3`, uuid.UUID(a.OrgID), id, user).Scan(&previousRole)
		if oldErr != nil && !errors.Is(oldErr, pgx.ErrNoRows) {
			return oldErr
		}
		if previousRole == "space_admin" && role == "member" {
			var admins int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE org_id=$1 AND space_id=$2 AND role='space_admin'`, uuid.UUID(a.OrgID), id).Scan(&admins); err != nil {
				return err
			}
			if admins <= 1 {
				return apperr.Conflict("last_space_admin", "Assign another Space Admin first.")
			}
		}
		tag, err := tx.Exec(ctx, `INSERT INTO space_members (org_id,space_id,user_id,role,added_by)
            VALUES($1,$2,$3,$4,$5)
            ON CONFLICT (space_id,user_id) DO UPDATE SET role=excluded.role,added_by=excluded.added_by`,
			uuid.UUID(a.OrgID), id, user, role, uuid.UUID(a.UserID))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound()
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.member_changed", TargetType: "space", TargetID: id.String(),
			Metadata: map[string]any{"user_id": user.String(), "role": role}})
	})
}

func (s *Service) RemoveSpaceMember(ctx context.Context, a tenant.Actor, id, user uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM spaces WHERE org_id=$1 AND id=$2 FOR UPDATE`, uuid.UUID(a.OrgID), id); err != nil {
			return err
		}
		sp, err := s.space(ctx, tx, a, id)
		if err != nil {
			return err
		}
		if user != uuid.UUID(a.UserID) {
			if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceMembersManage); err != nil {
				return err
			}
		} else if !sp.Joined {
			return apperr.NotFound()
		}
		var role string
		err = tx.QueryRow(ctx, `SELECT role FROM space_members WHERE org_id=$1 AND space_id=$2 AND user_id=$3`,
			uuid.UUID(a.OrgID), id, user).Scan(&role)
		if err != nil {
			return missing(err)
		}
		if role == "space_admin" {
			var count int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE org_id=$1 AND space_id=$2 AND role='space_admin'`,
				uuid.UUID(a.OrgID), id).Scan(&count); err != nil {
				return err
			}
			if count <= 1 {
				return apperr.Conflict("last_space_admin", "Assign another Space Admin before removing the last one.")
			}
		}
		_, err = tx.Exec(ctx, `DELETE FROM space_members WHERE org_id=$1 AND space_id=$2 AND user_id=$3`,
			uuid.UUID(a.OrgID), id, user)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE conversation_members cm SET left_at=now() FROM conversations c
            WHERE cm.org_id=$1 AND c.org_id=cm.org_id AND c.id=cm.conversation_id
              AND c.space_id=$2 AND cm.user_id=$3 AND c.kind='channel' AND cm.left_at IS NULL`,
			uuid.UUID(a.OrgID), id, user)
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "space.member_removed", TargetType: "space", TargetID: id.String(),
			Metadata: map[string]any{"user_id": user.String()}})
	})
}

// RemoveMySpaceMembership leaves a public Space without changing its other members.
func (s *Service) LeaveSpace(ctx context.Context, a tenant.Actor, id uuid.UUID, meta httpx.ClientMeta) error {
	return s.RemoveSpaceMember(ctx, a, id, uuid.UUID(a.UserID), meta)
}

func isConflict(err error) bool   { return unique(err) }
func noContent(v *string) *string { return optionalText(strings.TrimSpace(textValue(v))) }
