package authz

import (
	"testing"

	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

var roles = []tenant.Role{tenant.RoleOwner, tenant.RoleAdmin, tenant.RoleMember, tenant.RoleGuest}

// The role × permission matrix (MULTITENANCY.md §3.3) under default settings. A policy change
// without a matching change here fails the build.
var matrix = map[Permission][4]bool{ // owner, admin, member, guest
	OrgView:              {true, true, true, true},
	OrgSettingsManage:    {true, true, false, false},
	OrgSecurityManage:    {true, true, false, false},
	OrgDirectoryView:     {true, true, true, false},
	OrgMembersInvite:     {true, true, false, false},
	OrgGuestsInvite:      {true, true, false, false},
	OrgInvitationsView:   {true, true, false, false},
	OrgMembersManage:     {true, true, false, false},
	OrgAdminsManage:      {true, false, false, false},
	OrgOwnershipTransfer: {true, false, false, false},
	OrgDelete:            {true, false, false, false},
	AuditView:            {true, true, false, false},
	AuditExport:          {true, true, false, false},
}

// The catalogue served to the Roles & Permissions view lists exactly the permissions above.
func TestCatalogueCoversEveryPermission(t *testing.T) {
	seen := map[Permission]bool{}
	for _, p := range Catalogue() {
		if seen[p.ID] || p.Description == "" {
			t.Fatalf("duplicate or undescribed permission %q", p.ID)
		}
		seen[p.ID] = true
		if _, ok := matrix[p.ID]; !ok {
			t.Errorf("%s is in the catalogue but not in the matrix", p.ID)
		}
	}
	for p := range matrix {
		if !seen[p] {
			t.Errorf("%s is missing from the catalogue", p)
		}
	}
}

func TestPermissionMatrix(t *testing.T) {
	s := DefaultSettings()
	for perm, row := range matrix {
		for i, role := range roles {
			if got := Allowed(role, s, perm); got != row[i] {
				t.Errorf("%s as %s: got %v, want %v", perm, role, got, row[i])
			}
		}
	}
	if Allowed("superuser", s, OrgView) || Allowed(tenant.RoleOwner, s, "unknown.permission") {
		t.Fatal("unknown roles and permissions must be denied")
	}
}

func TestSettingsRefinePermissions(t *testing.T) {
	members := Settings{InvitePermission: "members", AllowGuestInvites: true}
	if !Allowed(tenant.RoleMember, members, OrgMembersInvite) || !Allowed(tenant.RoleMember, members, OrgInvitationsView) {
		t.Fatal("members may invite when the organisation allows it")
	}
	if Allowed(tenant.RoleGuest, members, OrgMembersInvite) {
		t.Fatal("guests never invite")
	}
	if Allowed(tenant.RoleMember, members, OrgGuestsInvite) {
		t.Fatal("only Owner/Admin may invite guests")
	}
	noGuests := Settings{InvitePermission: "admins", AllowGuestInvites: false}
	if Allowed(tenant.RoleOwner, noGuests, OrgGuestsInvite) {
		t.Fatal("guest invitations disabled by policy")
	}
}

func TestInvitePermissionFor(t *testing.T) {
	cases := map[tenant.Role]Permission{tenant.RoleAdmin: OrgAdminsManage, tenant.RoleMember: OrgMembersInvite, tenant.RoleGuest: OrgGuestsInvite}
	for role, want := range cases {
		if got, ok := InvitePermissionFor(role); !ok || got != want {
			t.Errorf("%s: %s", role, got)
		}
	}
	if _, ok := InvitePermissionFor(tenant.RoleOwner); ok {
		t.Fatal("inviting as Owner must be impossible")
	}
}

func actor(role tenant.Role) tenant.Actor {
	return tenant.Actor{UserID: tenant.UserID(uuid.New()), OrgID: tenant.OrgID(uuid.New()), Role: role}
}

func code(err error) string {
	if err == nil {
		return "allow"
	}
	e, _ := apperr.As(err)
	return e.Code
}

func TestMemberChangeRules(t *testing.T) {
	s := DefaultSettings()
	other := tenant.UserID(uuid.New())
	cases := []struct {
		name   string
		actor  tenant.Role
		change MemberChange
		want   string
	}{
		{"admin suspends member", tenant.RoleAdmin, MemberChange{TargetUserID: other, TargetRole: tenant.RoleMember, NewStatus: "suspended"}, "allow"},
		{"admin turns member into guest", tenant.RoleAdmin, MemberChange{TargetUserID: other, TargetRole: tenant.RoleMember, NewRole: tenant.RoleGuest}, "allow"},
		{"admin cannot touch admin", tenant.RoleAdmin, MemberChange{TargetUserID: other, TargetRole: tenant.RoleAdmin, NewStatus: "deactivated"}, "forbidden"},
		{"admin cannot promote to admin", tenant.RoleAdmin, MemberChange{TargetUserID: other, TargetRole: tenant.RoleMember, NewRole: tenant.RoleAdmin}, "forbidden"},
		{"owner promotes to admin", tenant.RoleOwner, MemberChange{TargetUserID: other, TargetRole: tenant.RoleMember, NewRole: tenant.RoleAdmin}, "allow"},
		{"owner demotes admin", tenant.RoleOwner, MemberChange{TargetUserID: other, TargetRole: tenant.RoleAdmin, NewRole: tenant.RoleMember}, "allow"},
		{"nobody changes the owner", tenant.RoleOwner, MemberChange{TargetUserID: other, TargetRole: tenant.RoleOwner, NewStatus: "suspended"}, "owner_protected"},
		{"no role becomes owner", tenant.RoleOwner, MemberChange{TargetUserID: other, TargetRole: tenant.RoleAdmin, NewRole: tenant.RoleOwner}, "ownership_transfer_required"},
		{"member cannot manage", tenant.RoleMember, MemberChange{TargetUserID: other, TargetRole: tenant.RoleGuest, NewStatus: "suspended"}, "forbidden"},
		{"guest cannot manage", tenant.RoleGuest, MemberChange{TargetUserID: other, TargetRole: tenant.RoleGuest, NewStatus: "suspended"}, "forbidden"},
	}
	for _, tc := range cases {
		if got := code(CheckMemberChange(actor(tc.actor), s, tc.change)); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
	self := actor(tenant.RoleOwner)
	if got := code(CheckMemberChange(self, s, MemberChange{TargetUserID: self.UserID, TargetRole: tenant.RoleOwner})); got != "cannot_modify_self" {
		t.Errorf("self change: %s", got)
	}
}

func TestInvitationRevokeRules(t *testing.T) {
	s := Settings{InvitePermission: "members", AllowGuestInvites: true}
	member := actor(tenant.RoleMember)
	if err := CheckInvitationRevoke(member, s, member.UserID, tenant.RoleMember); err != nil {
		t.Fatal("inviter may revoke their own invitation")
	}
	if CheckInvitationRevoke(member, s, tenant.UserID(uuid.New()), tenant.RoleMember) == nil {
		t.Fatal("member revoked someone else's invitation")
	}
	if CheckInvitationRevoke(actor(tenant.RoleAdmin), s, tenant.UserID(uuid.New()), tenant.RoleAdmin) == nil {
		t.Fatal("admin revoked another admin's Admin invitation")
	}
	if CheckInvitationRevoke(actor(tenant.RoleAdmin), s, tenant.UserID(uuid.New()), tenant.RoleGuest) != nil {
		t.Fatal("admin may revoke guest invitations")
	}
}
