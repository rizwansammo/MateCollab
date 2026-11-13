//go:build integration

package collaboration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

type collabFixture struct {
	svc                         *Service
	owner, member, guest, other tenant.Actor
	database                    *testkit.Database
}

func newCollabFixture(t *testing.T) collabFixture {
	t.Helper()
	data := testkit.NewDatabase(t)
	orgA, orgB := uuid.New(), uuid.New()
	people := []struct {
		id      uuid.UUID
		display string
		org     uuid.UUID
		role    string
	}{
		{uuid.New(), "Owner Alpha", orgA, "owner"}, {uuid.New(), "Member Alpha", orgA, "member"},
		{uuid.New(), "Guest Alpha", orgA, "guest"}, {uuid.New(), "Owner Beta", orgB, "owner"},
	}
	ctx := context.Background()
	for _, org := range []struct {
		id   uuid.UUID
		slug string
	}{
		{orgA, "acme-collaboration"}, {orgB, "beta-collaboration"},
	} {
		if _, err := data.Owner.Exec(ctx, `INSERT INTO organisations(id,slug,name)
            VALUES($1,$2,$3)`, org.id, org.slug, org.slug); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range people {
		if _, err := data.Owner.Exec(ctx, `INSERT INTO users(id,display_name) VALUES($1,$2)`, p.id, p.display); err != nil {
			t.Fatal(err)
		}
		if _, err := data.Owner.Exec(ctx, `INSERT INTO organisation_memberships(org_id,user_id,role) VALUES($1,$2,$3)`,
			p.org, p.id, p.role); err != nil {
			t.Fatal(err)
		}
	}
	actor := func(i int) tenant.Actor {
		return tenant.Actor{UserID: tenant.UserID(people[i].id), OrgID: tenant.OrgID(people[i].org), Role: tenant.Role(people[i].role)}
	}
	return collabFixture{svc: New(data.App), owner: actor(0), member: actor(1), guest: actor(2), other: actor(3), database: data}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok || e.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
}

func TestPhase3SpaceAndChannelIsolation(t *testing.T) {
	f := newCollabFixture(t)
	ctx := context.Background()
	meta := httpx.ClientMeta{}
	public, err := f.svc.CreateSpace(ctx, f.owner, SpaceInput{Name: "Engineering", Kind: "team", Visibility: "public"}, meta)
	if err != nil {
		t.Fatal(err)
	}
	if !public.Joined || public.ViewerRole != "space_admin" {
		t.Fatalf("creator must administer Space: %+v", public)
	}
	channels, err := f.svc.ListChannels(ctx, f.owner, public.ID)
	if err != nil || len(channels) != 1 || channels[0].Name != "general" || !channels[0].IsDefault {
		t.Fatalf("missing default Channel: %+v, %v", channels, err)
	}

	list, err := f.svc.ListSpaces(ctx, f.member)
	if err != nil || len(list) != 1 || list[0].Joined {
		t.Fatalf("public Space browsing incorrect: %+v, %v", list, err)
	}
	if _, err = f.svc.ListChannels(ctx, f.member, public.ID); err == nil {
		t.Fatal("unjoined member read Channels")
	}
	if err = f.svc.JoinSpace(ctx, f.member, public.ID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.ListChannels(ctx, f.member, public.ID); err != nil {
		t.Fatal(err)
	}

	list, err = f.svc.ListSpaces(ctx, f.guest)
	if err != nil || len(list) != 0 {
		t.Fatalf("ungranted guest discovered Spaces: %+v %v", list, err)
	}
	requireCode(t, f.svc.JoinSpace(ctx, f.guest, public.ID, meta), "not_found")
	if err = f.svc.SetSpaceMember(ctx, f.owner, public.ID, uuid.UUID(f.guest.UserID), "member", meta); err != nil {
		t.Fatal(err)
	}
	if list, err = f.svc.ListSpaces(ctx, f.guest); err != nil || len(list) != 1 {
		t.Fatalf("granted guest cannot see Space: %v %v", list, err)
	}
	if _, err = f.svc.ListChannels(ctx, f.guest, public.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.CreateChannel(ctx, f.guest, public.ID, ChannelInput{Name: "not-allowed"}, meta); err == nil {
		t.Fatal("guest created Channel")
	}

	secret, err := f.svc.CreateChannel(ctx, f.member, public.ID, ChannelInput{Name: "private-planning", Visibility: "private"}, meta)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Channel(ctx, f.guest, public.ID, secret.ID); err == nil {
		t.Fatal("private Channel leaked to guest")
	}
	if err = f.svc.AddChannelMember(ctx, f.member, public.ID, secret.ID, uuid.UUID(f.guest.UserID), meta); err != nil {
		t.Fatal(err)
	}
	if _, err = f.svc.Channel(ctx, f.guest, public.ID, secret.ID); err != nil {
		t.Fatal(err)
	}
	requireCode(t, f.svc.AddChannelMember(ctx, f.guest, public.ID, secret.ID, uuid.UUID(f.owner.UserID), meta), "forbidden")

	requireCode(t, func() error { _, err := f.svc.Space(ctx, f.other, public.ID); return err }(), "not_found")
	requireCode(t, func() error { _, err := f.svc.Channel(ctx, f.other, public.ID, secret.ID); return err }(), "not_found")
	requireCode(t, f.svc.DeleteSpace(ctx, f.member, public.ID, meta), "forbidden")
}

func TestPhase3PrivateSpaceLifecycleAndPolicy(t *testing.T) {
	f := newCollabFixture(t)
	ctx := context.Background()
	meta := httpx.ClientMeta{}
	private, err := f.svc.CreateSpace(ctx, f.owner, SpaceInput{Name: "Finance", Kind: "project", Visibility: "private"}, meta)
	if err != nil {
		t.Fatal(err)
	}
	if list, err := f.svc.ListSpaces(ctx, f.member); err != nil || len(list) != 0 {
		t.Fatalf("private Space leaked: %v %v", list, err)
	}
	requireCode(t, func() error { _, err := f.svc.Space(ctx, f.member, private.ID); return err }(), "not_found")
	if err := f.svc.SetSpaceMember(ctx, f.owner, private.ID, uuid.UUID(f.member.UserID), "member", meta); err != nil {
		t.Fatal(err)
	}
	if list, err := f.svc.ListSpaces(ctx, f.member); err != nil || len(list) != 1 {
		t.Fatalf("private membership not applied: %v %v", list, err)
	}
	if err := f.svc.LeaveSpace(ctx, f.member, private.ID, meta); err != nil {
		t.Fatal(err)
	}
	if list, err := f.svc.ListSpaces(ctx, f.member); err != nil || len(list) != 0 {
		t.Fatalf("private Space still visible after leave: %v %v", list, err)
	}

	// Policies apply to Members but do not block Owners.
	if _, err := f.database.Owner.Exec(ctx, `INSERT INTO organisation_settings (org_id,section,settings)
       VALUES($1,'messaging','{"public_space_creation":"admins","private_space_creation":"admins"}')
       ON CONFLICT(org_id,section) DO UPDATE SET settings=EXCLUDED.settings`, uuid.UUID(f.owner.OrgID)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateSpace(ctx, f.member, SpaceInput{Name: "Member Room", Visibility: "public"}, meta); err == nil {
		t.Fatal("Member ignored Space creation restriction")
	}
	if _, err := f.svc.CreateSpace(ctx, f.owner, SpaceInput{Name: "Owner Room", Visibility: "public"}, meta); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveSpace(ctx, f.owner, private.ID, meta); err != nil {
		t.Fatal(err)
	}
	if list, err := f.svc.ListSpaces(ctx, f.owner); err != nil || len(list) != 1 {
		t.Fatalf("archived Space listed: %+v %v", list, err)
	}
	if err := f.svc.DeleteSpace(ctx, f.owner, private.ID, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Space(ctx, f.owner, private.ID); err == nil {
		t.Fatal("deleted Space still readable")
	}
}
