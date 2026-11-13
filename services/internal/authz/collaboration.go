package authz

import (
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// SpaceFacts are trusted facts loaded inside an organisation-scoped transaction.
// Resource checks here are shared by all Space and Channel service operations.
type SpaceFacts struct {
	Public   bool
	Joined   bool
	Admin    bool
	Archived bool
}

type ChannelFacts struct {
	Public   bool
	Member   bool
	Creator  bool
	Archived bool
}

type SpaceAction string

const (
	SpaceDiscover      SpaceAction = "discover"
	SpaceRead          SpaceAction = "read"
	SpaceJoin          SpaceAction = "join"
	SpaceLeave         SpaceAction = "leave"
	SpaceUpdate        SpaceAction = "update"
	SpaceArchive       SpaceAction = "archive"
	SpaceMembersManage SpaceAction = "members.manage"
	SpaceChannelCreate SpaceAction = "channel.create"
)

// CanCreateSpace is constrained by the organisation's messaging policy.
// Admins may always create; Guests never create.
func CanCreateSpace(a tenant.Actor, s Settings, private bool) bool {
	if !a.Role.Valid() || a.Role == tenant.RoleGuest {
		return false
	}
	if isAdmin(a.Role) {
		return true
	}
	if private {
		return s.PrivateSpaceCreation == "all"
	}
	return s.PublicSpaceCreation == "all"
}

// CanDiscoverSpace returns metadata visibility, not permission to read channel content.
func CanDiscoverSpace(a tenant.Actor, f SpaceFacts) bool {
	if !a.Role.Valid() {
		return false
	}
	return f.Joined || (a.Role != tenant.RoleGuest && (isAdmin(a.Role) || f.Public))
}

// CheckSpace implements the resource-level Space permissions in one place.
func CheckSpace(a tenant.Actor, f SpaceFacts, action SpaceAction) error {
	if !CanDiscoverSpace(a, f) {
		return apperr.NotFound()
	}
	if action == SpaceDiscover {
		return nil
	}
	if action == SpaceRead {
		if f.Joined || isAdmin(a.Role) {
			return nil
		}
		return ErrForbidden("space_membership_required")
	}
	if f.Archived {
		return apperr.Conflict("space_archived", "This Space is archived.")
	}
	switch action {
	case SpaceJoin:
		if !f.Joined && (isAdmin(a.Role) || (f.Public && a.Role != tenant.RoleGuest)) {
			return nil
		}
	case SpaceLeave:
		if f.Joined {
			return nil
		}
	case SpaceChannelCreate:
		if f.Joined && a.Role != tenant.RoleGuest {
			return nil
		}
	case SpaceUpdate, SpaceArchive, SpaceMembersManage:
		if a.Role != tenant.RoleGuest && (isAdmin(a.Role) || f.Admin) {
			return nil
		}
	}
	return ErrForbidden("forbidden")
}

// CanReadChannel is the reusable conversation read predicate for Phase 3 channels.
func CanReadChannel(a tenant.Actor, space SpaceFacts, channel ChannelFacts) bool {
	if !a.Role.Valid() || space.Archived || channel.Archived {
		return false
	}
	if !space.Joined {
		return false
	}
	if channel.Public {
		return true
	}
	return channel.Member
}

type ChannelAction string

const (
	ChannelRead          ChannelAction = "read"
	ChannelUpdate        ChannelAction = "update"
	ChannelArchive       ChannelAction = "archive"
	ChannelMembersManage ChannelAction = "members.manage"
)

// CheckChannel hides private channel existence from non-members even if they administer a Space.
func CheckChannel(a tenant.Actor, space SpaceFacts, channel ChannelFacts, action ChannelAction) error {
	if !CanReadChannel(a, space, channel) {
		return apperr.NotFound()
	}
	if action == ChannelRead {
		return nil
	}
	if channel.Archived {
		return apperr.Conflict("channel_archived", "This Channel is archived.")
	}
	switch action {
	case ChannelUpdate, ChannelArchive:
		if a.Role != tenant.RoleGuest && (isAdmin(a.Role) || space.Admin || channel.Creator) {
			return nil
		}
	case ChannelMembersManage:
		if a.Role != tenant.RoleGuest && !channel.Public && (isAdmin(a.Role) || space.Admin || channel.Creator || channel.Member) {
			return nil
		}
	}
	return ErrForbidden("forbidden")
}
