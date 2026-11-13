package authz

import (
	"github.com/google/uuid"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"testing"
)

func TestSpaceCreationPolicy(t *testing.T) {
	for _, tc := range []struct {
		role                          tenant.Role
		public, private               string
		publicAllowed, privateAllowed bool
	}{
		{tenant.RoleOwner, "admins", "admins", true, true},
		{tenant.RoleAdmin, "admins", "admins", true, true},
		{tenant.RoleMember, "all", "admins", true, false},
		{tenant.RoleMember, "admins", "all", false, true},
		{tenant.RoleGuest, "all", "all", false, false},
	} {
		a := tenant.Actor{Role: tc.role}
		s := DefaultSettings()
		s.PublicSpaceCreation = tc.public
		s.PrivateSpaceCreation = tc.private
		if got := CanCreateSpace(a, s, false); got != tc.publicAllowed {
			t.Errorf("%s public: %v", tc.role, got)
		}
		if got := CanCreateSpace(a, s, true); got != tc.privateAllowed {
			t.Errorf("%s private: %v", tc.role, got)
		}
	}
}

func TestSpacesNeverExposeUnjoinedGuestAccess(t *testing.T) {
	guest := tenant.Actor{Role: tenant.RoleGuest}
	member := tenant.Actor{Role: tenant.RoleMember}
	admin := tenant.Actor{Role: tenant.RoleAdmin}
	public := SpaceFacts{Public: true}
	private := SpaceFacts{Public: false}
	if CanDiscoverSpace(guest, public) || CanDiscoverSpace(guest, private) {
		t.Fatal("ungranted guest discovered a Space")
	}
	if !CanDiscoverSpace(member, public) || CanDiscoverSpace(member, private) {
		t.Fatal("Member Space discovery incorrect")
	}
	if !CanDiscoverSpace(admin, private) {
		t.Fatal("admin cannot manage private Space")
	}
	if CheckSpace(guest, public, SpaceJoin) == nil {
		t.Fatal("guest could join a public Space")
	}
	guestGranted := SpaceFacts{Public: true, Joined: true}
	if CheckSpace(guest, guestGranted, SpaceChannelCreate) == nil {
		t.Fatal("guest could create Channel")
	}
	guestAdmin := SpaceFacts{Public: false, Joined: true, Admin: true}
	if CheckSpace(guest, guestAdmin, SpaceMembersManage) == nil {
		t.Fatal("guest elevated through Space Admin membership")
	}
}

func TestChannelReadAndMemberManagement(t *testing.T) {
	guest := tenant.Actor{Role: tenant.RoleGuest, UserID: tenant.UserID(uuid.New())}
	admin := tenant.Actor{Role: tenant.RoleAdmin}
	joined := SpaceFacts{Public: true, Joined: true}
	notJoined := SpaceFacts{Public: true}
	public := ChannelFacts{Public: true}
	private := ChannelFacts{Public: false}
	if CanReadChannel(guest, notJoined, public) {
		t.Fatal("public Channel leaked to ungranted guest")
	}
	if !CanReadChannel(guest, joined, public) {
		t.Fatal("guest cannot read granted public Channel")
	}
	if CanReadChannel(guest, joined, private) {
		t.Fatal("private Channel leaked to Space member")
	}
	private.Member = true
	if !CanReadChannel(guest, joined, private) {
		t.Fatal("private Channel should be visible to explicit member")
	}
	if CheckChannel(guest, joined, private, ChannelMembersManage) == nil {
		t.Fatal("guest invited someone to private Channel")
	}
	if CheckChannel(admin, notJoined, public, ChannelRead) == nil {
		t.Fatal("admin read Channel before joining Space")
	}
	if CheckChannel(admin, joined, private, ChannelRead) != nil {
		t.Fatal("admin cannot read their own private Channel membership")
	}
}
