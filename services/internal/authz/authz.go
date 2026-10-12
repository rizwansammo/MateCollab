// Package authz is the single place where organisation permission decisions are made
// (MULTITENANCY.md §3–4). Policies are pure functions of the actor's role, the organisation's
// settings and resource facts; handlers never inspect roles.
package authz

import (
	"context"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Permission is a named capability from the catalogue (MULTITENANCY.md §3.3).
type Permission string

// Organisation permissions implemented so far. Space, messaging, meeting and file permissions
// are added with those modules.
const (
	OrgView              Permission = "org.view"
	OrgSettingsManage    Permission = "org.settings.manage"
	OrgSecurityManage    Permission = "org.security.manage"
	OrgDirectoryView     Permission = "org.directory.view"
	OrgMembersInvite     Permission = "org.members.invite"
	OrgGuestsInvite      Permission = "org.guests.invite"
	OrgInvitationsView   Permission = "org.invitations.view"
	OrgMembersManage     Permission = "org.members.manage"
	OrgAdminsManage      Permission = "org.admins.manage"
	OrgOwnershipTransfer Permission = "org.ownership.transfer"
	OrgDelete            Permission = "org.delete"
	AuditView            Permission = "audit.view"
)

// Settings are the organisation settings that refine role permissions (the `security` section).
type Settings struct {
	InvitePermission  string // "admins" (default) or "members"
	AllowGuestInvites bool
}

// DefaultSettings are used until an organisation changes them.
func DefaultSettings() Settings { return Settings{InvitePermission: "admins", AllowGuestInvites: true} }

func isAdmin(r tenant.Role) bool { return r == tenant.RoleOwner || r == tenant.RoleAdmin }

// Allowed reports whether role may use permission p under settings s.
func Allowed(role tenant.Role, s Settings, p Permission) bool {
	switch p {
	case OrgView:
		return role.Valid()
	case OrgSettingsManage, OrgSecurityManage, OrgMembersManage, AuditView:
		return isAdmin(role)
	case OrgDirectoryView:
		return isAdmin(role) || role == tenant.RoleMember
	case OrgMembersInvite, OrgInvitationsView:
		return isAdmin(role) || (role == tenant.RoleMember && s.InvitePermission == "members")
	case OrgGuestsInvite:
		return isAdmin(role) && s.AllowGuestInvites
	case OrgAdminsManage, OrgOwnershipTransfer, OrgDelete:
		return role == tenant.RoleOwner
	}
	return false
}

// ErrForbidden is the standard 403 for a visible resource the actor may not act on.
func ErrForbidden(code string) error {
	return apperr.Forbidden(code, "You don't have permission to do this.")
}

// Check returns nil when the actor may use p, otherwise a 403 problem.
func Check(a tenant.Actor, s Settings, p Permission) error {
	if Allowed(a.Role, s, p) {
		return nil
	}
	return ErrForbidden("forbidden")
}

// InvitePermissionFor is the permission needed to invite someone with role r.
func InvitePermissionFor(r tenant.Role) (Permission, bool) {
	switch r {
	case tenant.RoleAdmin:
		return OrgAdminsManage, true
	case tenant.RoleMember:
		return OrgMembersInvite, true
	case tenant.RoleGuest:
		return OrgGuestsInvite, true
	}
	return "", false // nobody is invited as Owner
}

// MemberChange describes a requested change to another member.
type MemberChange struct {
	TargetUserID tenant.UserID
	TargetRole   tenant.Role
	NewRole      tenant.Role // "" = unchanged
	NewStatus    string      // "" = unchanged
}

// CheckMemberChange applies the member-management rules: nobody changes themselves or the
// Owner (ownership moves only by transfer), Admins manage Members and Guests, and only the
// Owner grants, revokes or manages Admins.
func CheckMemberChange(a tenant.Actor, s Settings, c MemberChange) error {
	if c.TargetUserID == a.UserID {
		return ErrForbidden("cannot_modify_self")
	}
	if c.TargetRole == tenant.RoleOwner {
		return ErrForbidden("owner_protected")
	}
	if c.NewRole == tenant.RoleOwner {
		return ErrForbidden("ownership_transfer_required")
	}
	need := OrgMembersManage
	if c.TargetRole == tenant.RoleAdmin || c.NewRole == tenant.RoleAdmin {
		need = OrgAdminsManage
	}
	return Check(a, s, need)
}

// CheckInvitationRevoke: the inviter may revoke their own invitation; otherwise managing
// members (or, for Admin invitations, managing Admins) is required.
func CheckInvitationRevoke(a tenant.Actor, s Settings, invitedBy tenant.UserID, role tenant.Role) error {
	if invitedBy == a.UserID {
		return nil
	}
	if role == tenant.RoleAdmin {
		return Check(a, s, OrgAdminsManage)
	}
	return Check(a, s, OrgMembersManage)
}

type settingsKey struct{}

// WithSettings stores the organisation's settings for the current request.
func WithSettings(ctx context.Context, s Settings) context.Context {
	return context.WithValue(ctx, settingsKey{}, s)
}

// SettingsFrom returns the request's organisation settings (defaults when absent).
func SettingsFrom(ctx context.Context) Settings {
	if s, ok := ctx.Value(settingsKey{}).(Settings); ok {
		return s
	}
	return DefaultSettings()
}
