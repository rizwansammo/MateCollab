package collaboration

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

type Channel struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	SpaceID    uuid.UUID  `json:"space_id"`
	Name       string     `json:"name"`
	Topic      *string    `json:"topic"`
	Visibility string     `json:"visibility"`
	IsDefault  bool       `json:"is_default"`
	CreatedBy  uuid.UUID  `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"`
	Member     bool       `json:"member"`
}

type ChannelInput struct {
	Name       string  `json:"name"`
	Topic      *string `json:"topic"`
	Visibility string  `json:"visibility"`
}

type ChannelMember struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	JoinedAt    time.Time `json:"joined_at"`
}

func validateChannel(in ChannelInput) (ChannelInput, error) {
	in.Name = strings.ToLower(strings.TrimSpace(in.Name))
	if len(in.Name) > 80 || !channelName.MatchString(in.Name) || strings.Contains(in.Name, "--") {
		return in, apperr.Validation(apperr.Field("name", "invalid", "Use lowercase letters, numbers and single hyphens (1 to 80 characters)."))
	}
	if in.Visibility == "" {
		in.Visibility = "public"
	}
	if in.Visibility != "public" && in.Visibility != "private" {
		return in, apperr.Validation(apperr.Field("visibility", "invalid", "Choose Public or Private."))
	}
	if in.Topic != nil && len([]rune(*in.Topic)) > 250 {
		return in, apperr.Validation(apperr.Field("topic", "too_long", "Use 250 characters or fewer."))
	}
	return in, nil
}

func scanChannel(row pgx.Row) (Channel, error) {
	var c Channel
	var member *uuid.UUID
	err := row.Scan(&c.ID, &c.OrgID, &c.SpaceID, &c.Name, &c.Topic, &c.Visibility, &c.IsDefault,
		&c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt, &member)
	c.Member = member != nil
	return c, err
}

const selectChannel = `SELECT c.id,c.org_id,c.space_id,c.name,c.topic,c.visibility,c.is_default,c.created_by,
    c.created_at,c.updated_at,c.archived_at,cm.user_id
    FROM conversations c LEFT JOIN conversation_members cm
    ON cm.org_id=c.org_id AND cm.conversation_id=c.id AND cm.user_id=$4 AND cm.left_at IS NULL
    WHERE c.org_id=$1 AND c.space_id=$2 AND c.id=$3 AND c.kind='channel'`

func channelFacts(c Channel, a tenant.Actor) authz.ChannelFacts {
	return authz.ChannelFacts{Public: c.Visibility == "public", Member: c.Member,
		Creator: c.CreatedBy == uuid.UUID(a.UserID), Archived: c.ArchivedAt != nil}
}

func (s *Service) channel(ctx context.Context, tx *db.Tx, a tenant.Actor, spaceID, channelID uuid.UUID) (Channel, error) {
	c, err := scanChannel(tx.QueryRow(ctx, selectChannel, uuid.UUID(a.OrgID), spaceID, channelID, uuid.UUID(a.UserID)))
	return c, missing(err)
}

func (s *Service) Channel(ctx context.Context, a tenant.Actor, spaceID, channelID uuid.UUID) (Channel, error) {
	var out Channel
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceDiscover); err != nil {
			return err
		}
		out, err = s.channel(ctx, tx, a, spaceID, channelID)
		if err != nil {
			return err
		}
		return authz.CheckChannel(a, spaceFacts(sp), channelFacts(out, a), authz.ChannelRead)
	})
	return out, err
}

// ListChannels never returns private Channel metadata to non-members.
// Unjoined public Space browsers can discover the Space but not its Channels.
func (s *Service) ListChannels(ctx context.Context, a tenant.Actor, spaceID uuid.UUID) ([]Channel, error) {
	out := []Channel{}
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceRead); err != nil {
			return err
		}
		if sp.ArchivedAt != nil {
			return nil
		}
		rows, err := tx.Query(ctx, `SELECT c.id,c.org_id,c.space_id,c.name,c.topic,c.visibility,c.is_default,c.created_by,
            c.created_at,c.updated_at,c.archived_at,cm.user_id
            FROM conversations c LEFT JOIN conversation_members cm
            ON cm.org_id=c.org_id AND cm.conversation_id=c.id AND cm.user_id=$3 AND cm.left_at IS NULL
            WHERE c.org_id=$1 AND c.space_id=$2 AND c.kind='channel' AND c.archived_at IS NULL
            AND (c.visibility='public' OR cm.user_id IS NOT NULL)
            ORDER BY c.is_default DESC,lower(c.name),c.id LIMIT 201`,
			uuid.UUID(a.OrgID), spaceID, uuid.UUID(a.UserID))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanChannel(rows)
			if err != nil {
				return err
			}
			if authz.CanReadChannel(a, spaceFacts(sp), channelFacts(c, a)) {
				out = append(out, c)
			}
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) CreateChannel(ctx context.Context, a tenant.Actor, spaceID uuid.UUID, in ChannelInput, meta httpx.ClientMeta) (Channel, error) {
	in, err := validateChannel(in)
	if err != nil {
		return Channel{}, err
	}
	if in.Name == "general" {
		return Channel{}, apperr.Conflict("reserved_channel", "Every Space already has a general Channel.")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Channel{}, err
	}
	var out Channel
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceChannelCreate); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO conversations (id,org_id,kind,space_id,name,topic,visibility,created_by)
            VALUES($1,$2,'channel',$3,$4,$5,$6,$7)`,
			id, uuid.UUID(a.OrgID), spaceID, in.Name, in.Topic, in.Visibility, uuid.UUID(a.UserID))
		if unique(err) {
			return conflictName()
		}
		if err != nil {
			return err
		}
		if in.Visibility == "private" {
			_, err = tx.Exec(ctx, `INSERT INTO conversation_members (org_id,conversation_id,user_id) VALUES($1,$2,$3)`,
				uuid.UUID(a.OrgID), id, uuid.UUID(a.UserID))
			if err != nil {
				return err
			}
		}
		out, err = s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "channel.created", TargetType: "channel", TargetID: id.String(),
			Metadata: map[string]any{"space_id": spaceID.String(), "visibility": in.Visibility}})
	})
	return out, err
}

func (s *Service) UpdateChannel(ctx context.Context, a tenant.Actor, spaceID, id uuid.UUID, in ChannelInput, expected time.Time, meta httpx.ClientMeta) (Channel, error) {
	in, err := validateChannel(in)
	if err != nil {
		return Channel{}, err
	}
	if in.Name == "general" {
		return Channel{}, apperr.Conflict("reserved_channel", "The general Channel cannot be renamed or reused.")
	}
	var out Channel
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceRead); err != nil {
			return err
		}
		before, err := s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		if err = authz.CheckChannel(a, spaceFacts(sp), channelFacts(before, a), authz.ChannelUpdate); err != nil {
			return err
		}
		if before.IsDefault {
			return apperr.Conflict("default_channel_protected", "The general Channel is always available.")
		}
		if before.UpdatedAt.UnixMicro() != expected.UnixMicro() {
			return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "This Channel changed. Reload and try again."}
		}
		tag, err := tx.Exec(ctx, `UPDATE conversations SET name=$4,topic=$5,visibility=$6,updated_at=now()
            WHERE org_id=$1 AND space_id=$2 AND id=$3 AND kind='channel' AND updated_at=$7`,
			uuid.UUID(a.OrgID), spaceID, id, in.Name, in.Topic, in.Visibility, expected)
		if unique(err) {
			return conflictName()
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return &apperr.Error{Kind: apperr.KindPreconditionFailed, Code: "version_mismatch", Detail: "This Channel changed. Reload and try again."}
		}
		if before.Visibility == "public" && in.Visibility == "private" {
			// Former participants must not silently retain access when a public Channel becomes private.
			if _, err = tx.Exec(ctx, `UPDATE conversation_members SET left_at=now()
                WHERE org_id=$1 AND conversation_id=$2 AND left_at IS NULL`, uuid.UUID(a.OrgID), id); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO conversation_members (org_id,conversation_id,user_id)
                VALUES($1,$2,$3) ON CONFLICT (conversation_id,user_id) DO UPDATE SET left_at=NULL`,
				uuid.UUID(a.OrgID), id, uuid.UUID(a.UserID))
			if err != nil {
				return err
			}
		}
		out, err = s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "channel.updated", TargetType: "channel", TargetID: id.String()})
	})
	return out, err
}

func (s *Service) ArchiveChannel(ctx context.Context, a tenant.Actor, spaceID, id uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		if err = authz.CheckSpace(a, spaceFacts(sp), authz.SpaceRead); err != nil {
			return err
		}
		ch, err := s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		if err = authz.CheckChannel(a, spaceFacts(sp), channelFacts(ch, a), authz.ChannelArchive); err != nil {
			return err
		}
		if ch.IsDefault {
			return apperr.Conflict("default_channel_protected", "The general Channel cannot be archived.")
		}
		_, err = tx.Exec(ctx, `UPDATE conversations SET archived_at=now(),updated_at=now()
            WHERE org_id=$1 AND space_id=$2 AND id=$3 AND kind='channel'`, uuid.UUID(a.OrgID), spaceID, id)
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "channel.archived", TargetType: "channel", TargetID: id.String()})
	})
}

func (s *Service) ListChannelMembers(ctx context.Context, a tenant.Actor, spaceID, id uuid.UUID) ([]ChannelMember, error) {
	out := []ChannelMember{}
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		ch, err := s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		if err = authz.CheckChannel(a, spaceFacts(sp), channelFacts(ch, a), authz.ChannelRead); err != nil {
			return err
		}
		if ch.Visibility != "private" {
			return apperr.NotFound()
		}
		rows, err := tx.Query(ctx, `SELECT cm.user_id,u.display_name,cm.joined_at
            FROM conversation_members cm JOIN users u ON u.id=cm.user_id
            JOIN organisation_memberships m ON m.org_id=cm.org_id AND m.user_id=cm.user_id AND m.status='active'
            WHERE cm.org_id=$1 AND cm.conversation_id=$2 AND cm.left_at IS NULL
            ORDER BY lower(u.display_name),cm.user_id LIMIT 201`, uuid.UUID(a.OrgID), id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m ChannelMember
			if err := rows.Scan(&m.UserID, &m.DisplayName, &m.JoinedAt); err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Service) AddChannelMember(ctx context.Context, a tenant.Actor, spaceID, id, user uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		ch, err := s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		if err = authz.CheckChannel(a, spaceFacts(sp), channelFacts(ch, a), authz.ChannelMembersManage); err != nil {
			return err
		}
		var active bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members sm JOIN organisation_memberships m
            ON m.org_id=sm.org_id AND m.user_id=sm.user_id AND m.status='active'
            WHERE sm.org_id=$1 AND sm.space_id=$2 AND sm.user_id=$3)`,
			uuid.UUID(a.OrgID), spaceID, user).Scan(&active)
		if err != nil {
			return err
		}
		if !active {
			return apperr.NotFound()
		}
		_, err = tx.Exec(ctx, `INSERT INTO conversation_members(org_id,conversation_id,user_id)
            VALUES($1,$2,$3) ON CONFLICT (conversation_id,user_id) DO UPDATE SET left_at=NULL`,
			uuid.UUID(a.OrgID), id, user)
		if err != nil {
			return err
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "channel.member_added", TargetType: "channel", TargetID: id.String(),
			Metadata: map[string]any{"user_id": user.String()}})
	})
}

func (s *Service) RemoveChannelMember(ctx context.Context, a tenant.Actor, spaceID, id, user uuid.UUID, meta httpx.ClientMeta) error {
	return db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		sp, err := s.space(ctx, tx, a, spaceID)
		if err != nil {
			return err
		}
		ch, err := s.channel(ctx, tx, a, spaceID, id)
		if err != nil {
			return err
		}
		if err = authz.CheckChannel(a, spaceFacts(sp), channelFacts(ch, a), authz.ChannelMembersManage); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE conversation_members SET left_at=now()
            WHERE org_id=$1 AND conversation_id=$2 AND user_id=$3 AND left_at IS NULL`,
			uuid.UUID(a.OrgID), id, user)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound()
		}
		return audit.RecordActor(ctx, tx, a, meta, audit.Entry{Action: "channel.member_removed", TargetType: "channel", TargetID: id.String(),
			Metadata: map[string]any{"user_id": user.String()}})
	})
}
