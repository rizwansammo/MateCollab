//go:build integration

package apiserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/emails"
)

func TestOrganisationsInvitationsAndAccess(t *testing.T) {
	h := newHarness(t, nil)
	orgA, ownerA := h.seedOrg("Acme", "acme", "owner@acme.test")
	orgB, ownerB := h.seedOrg("Globex", "globex", "owner@globex.test")
	ctx := context.Background()

	var admin, member, guest *account

	h.run("invite and accept as new accounts", func(t *testing.T) {
		_, adminToken := h.invite(ownerA.b, orgA, "admin@acme.test", "admin")
		_, memberToken := h.invite(ownerA.b, orgA, "member@acme.test", "member")
		_, guestToken := h.invite(ownerA.b, orgA, "guest@acme.test", "guest")

		preview := h.browser().do(http.MethodPost, "/invitations/preview", map[string]string{"token": memberToken}).expect(http.StatusOK).m()
		if preview["organisation"].(map[string]any)["name"] != "Acme" || preview["role"] != "member" ||
			preview["account_exists"] != false || preview["status"] != "pending" || preview["email"] != "member@acme.test" {
			t.Fatalf("preview: %v", preview)
		}

		admin = h.join(adminToken, "admin@acme.test", "Ada Admin")
		member = h.join(memberToken, "member@acme.test", "Mo Member")
		guest = h.join(guestToken, "guest@acme.test", "Gus Guest")
		doc := member.b.refresh()
		if list := doc["organisations"].([]any); len(list) != 1 || list[0].(map[string]any)["role"] != "member" {
			t.Fatalf("member organisations: %v", list)
		}
		if h.auditCount(orgA, "invitation.created") != 3 || h.auditCount(orgA, "invitation.accepted") != 3 {
			t.Fatal("invitation audit entries missing")
		}
	})

	h.run("invitation tokens are single use, unforgeable and carry no client-chosen role", func(t *testing.T) {
		_, token := h.invite(ownerA.b, orgA, "once@acme.test", "guest")
		anon := h.browser()
		// Unknown fields such as role or org_id are rejected outright.
		anon.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token, "display_name": "X", "password": "Strong-" + randomHex(t, 8), "role": "admin"}).
			expect(http.StatusBadRequest)
		anon.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token, "display_name": "X", "password": "Strong-" + randomHex(t, 8), "org_id": orgB}).
			expect(http.StatusBadRequest)
		anon.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token[:len(token)-2] + "xx", "display_name": "X", "password": "Strong-" + randomHex(t, 8)}).
			expect(http.StatusNotFound, "invitation_invalid")
		anon.do(http.MethodPost, "/invitations/preview", map[string]string{"token": ""}).expect(http.StatusNotFound, "invitation_invalid")
		once := h.join(token, "once@acme.test", "Once")
		var me struct {
			Role string `json:"role"`
		}
		uid := once.b.refresh()["user"].(map[string]any)["id"].(string)
		once.b.do(http.MethodGet, "/orgs/"+orgA+"/members/"+uid, nil).expect(http.StatusOK).json(&me)
		if me.Role != "guest" {
			t.Fatalf("role %q, want the invited role", me.Role)
		}
		h.browser().do(http.MethodPost, "/invitations/accept", map[string]string{"token": token, "display_name": "Again", "password": "Strong-" + randomHex(t, 8)}).
			expect(http.StatusConflict, "invitation_used")
		if p := h.browser().do(http.MethodPost, "/invitations/preview", map[string]string{"token": token}).expect(http.StatusOK).m(); p["status"] != "accepted" {
			t.Fatalf("preview of a used invitation: %v", p)
		}
	})

	h.run("expired, resent and revoked invitations", func(t *testing.T) {
		id, token := h.invite(ownerA.b, orgA, "late@acme.test", "member")
		if _, err := h.db.Owner.Exec(ctx, `UPDATE invitations SET expires_at = now() - interval '1 minute' WHERE id = $1`, id); err != nil {
			t.Fatal(err)
		}
		h.browser().do(http.MethodPost, "/invitations/accept", map[string]string{"token": token, "display_name": "Late", "password": "Strong-" + randomHex(t, 8)}).
			expect(http.StatusConflict, "invitation_expired")

		ownerA.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations/"+id+"/resend", nil).expect(http.StatusOK)
		fresh := h.lastToken("late@acme.test", emails.Invitation)
		if fresh == token {
			t.Fatal("resend must rotate the token")
		}
		h.browser().do(http.MethodPost, "/invitations/preview", map[string]string{"token": token}).expect(http.StatusNotFound, "invitation_invalid")
		h.browser().do(http.MethodPost, "/invitations/preview", map[string]string{"token": fresh}).expect(http.StatusOK)

		ownerA.b.do(http.MethodDelete, "/orgs/"+orgA+"/invitations/"+id, nil).expect(http.StatusNoContent)
		ownerA.b.do(http.MethodDelete, "/orgs/"+orgA+"/invitations/"+id, nil).expect(http.StatusNoContent) // idempotent
		h.browser().do(http.MethodPost, "/invitations/accept", map[string]string{"token": fresh, "display_name": "Late", "password": "Strong-" + randomHex(t, 8)}).
			expect(http.StatusNotFound, "invitation_invalid")
		ownerA.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations/"+id+"/resend", nil).expect(http.StatusConflict, "invitation_closed")

		// Duplicate pending invitations are refused; members cannot be re-invited.
		h.invite(ownerA.b, orgA, "dup@acme.test", "member")
		h.invite(ownerA.b, orgA, "pending@acme.test", "guest")
		ownerA.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "DUP@acme.test", "role": "member"}).expect(http.StatusConflict, "invitation_pending")
		ownerA.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": member.email, "role": "member"}).expect(http.StatusConflict, "already_member")

		var list struct {
			Data []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
			Page struct {
				NextCursor *string `json:"next_cursor"`
			} `json:"page"`
		}
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA+"/invitations?status=pending&limit=1", nil).expect(http.StatusOK).json(&list)
		if len(list.Data) != 1 || list.Data[0].Status != "pending" || list.Page.NextCursor == nil {
			t.Fatalf("pending page: %+v", list)
		}
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA+"/invitations?limit=1&cursor="+*list.Page.NextCursor, nil).expect(http.StatusBadRequest, "invalid_cursor") // bound to its filters
	})

	h.run("existing accounts accept while signed in as the invited address", func(t *testing.T) {
		_, token := h.invite(ownerA.b, orgA, ownerB.email, "member")
		anon := h.browser()
		if p := anon.do(http.MethodPost, "/invitations/preview", map[string]string{"token": token}).expect(http.StatusOK).m(); p["account_exists"] != true {
			t.Fatalf("preview: %v", p)
		}
		anon.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token, "display_name": "Imposter", "password": "Strong-" + randomHex(t, 8)}).
			expect(http.StatusUnauthorized, "sign_in_required")
		member.b.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token}).expect(http.StatusForbidden, "invitation_email_mismatch")
		ownerB.b.do(http.MethodPost, "/invitations/accept", map[string]string{"token": token}).expect(http.StatusOK)
		if list := ownerB.b.refresh()["organisations"].([]any); len(list) != 2 {
			t.Fatalf("owner B should now belong to two organisations: %v", list)
		}
		// Owner B's role differs per organisation: Owner of Globex, Member of Acme.
		ownerB.b.do(http.MethodGet, "/orgs/"+orgB+"/settings/security", nil).expect(http.StatusOK)
		ownerB.b.do(http.MethodGet, "/orgs/"+orgA+"/settings/security", nil).expect(http.StatusForbidden, "forbidden")
	})

	h.run("directory and roles", func(t *testing.T) {
		var dir struct {
			Data []struct {
				UserID string `json:"user_id"`
				Role   string `json:"role"`
			} `json:"data"`
			Page struct {
				NextCursor *string `json:"next_cursor"`
				HasMore    bool    `json:"has_more"`
			} `json:"page"`
		}
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA+"/members?limit=2", nil).expect(http.StatusOK).json(&dir)
		if len(dir.Data) != 2 || !dir.Page.HasMore {
			t.Fatalf("first page: %+v", dir)
		}
		seen := map[string]bool{}
		cur := ""
		for page := 0; page < 10; page++ {
			path := "/orgs/" + orgA + "/members?limit=2"
			if cur != "" {
				path += "&cursor=" + cur
			}
			dir.Page.NextCursor = nil
			ownerA.b.do(http.MethodGet, path, nil).expect(http.StatusOK).json(&dir)
			for _, m := range dir.Data {
				if seen[m.UserID] {
					t.Fatalf("member %s repeated across pages", m.UserID)
				}
				seen[m.UserID] = true
			}
			if dir.Page.NextCursor == nil {
				break
			}
			cur = *dir.Page.NextCursor
		}
		if len(seen) != 6 { // owner, admin, member, guest, once, owner B
			t.Fatalf("directory has %d members", len(seen))
		}
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA+"/members?role=member", nil).expect(http.StatusOK)
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA+"/members?role=superuser", nil).expect(http.StatusUnprocessableEntity)
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA+"/members?cursor=forged.cursor", nil).expect(http.StatusBadRequest, "invalid_cursor")
		member.b.do(http.MethodGet, "/orgs/"+orgA+"/members", nil).expect(http.StatusOK)
	})

	h.run("guest restrictions", func(t *testing.T) {
		guestID := guest.b.refresh()["user"].(map[string]any)["id"].(string)
		ownerID := ownerA.b.refresh()["user"].(map[string]any)["id"].(string)
		guest.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusOK)
		guest.b.do(http.MethodGet, "/orgs/"+orgA+"/members/"+guestID, nil).expect(http.StatusOK) // self
		guest.b.do(http.MethodGet, "/orgs/"+orgA+"/members", nil).expect(http.StatusForbidden, "forbidden")
		guest.b.do(http.MethodGet, "/orgs/"+orgA+"/members/"+ownerID, nil).expect(http.StatusForbidden, "forbidden")
		guest.b.do(http.MethodGet, "/orgs/"+orgA+"/invitations", nil).expect(http.StatusForbidden, "forbidden")
		guest.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "friend@acme.test", "role": "guest"}).expect(http.StatusForbidden, "forbidden")
		guest.b.do(http.MethodPatch, "/orgs/"+orgA, map[string]string{"name": "Pwned"}).expect(http.StatusForbidden, "forbidden")
		guest.b.do(http.MethodGet, "/orgs/"+orgA+"/settings/security", nil).expect(http.StatusForbidden, "forbidden")
	})

	h.run("member management rules", func(t *testing.T) {
		ids := map[string]string{}
		for name, a := range map[string]*account{"owner": ownerA, "admin": admin, "member": member, "guest": guest} {
			ids[name] = a.b.refresh()["user"].(map[string]any)["id"].(string)
		}
		path := func(who string) string { return "/orgs/" + orgA + "/members/" + ids[who] }

		member.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "x@acme.test", "role": "member"}).expect(http.StatusForbidden, "forbidden")
		member.b.do(http.MethodPatch, path("guest"), map[string]string{"status": "deactivated"}).expect(http.StatusForbidden, "forbidden")
		admin.b.do(http.MethodPatch, path("admin"), map[string]string{"role": "member"}).expect(http.StatusForbidden, "cannot_modify_self")
		admin.b.do(http.MethodPatch, path("owner"), map[string]string{"status": "deactivated"}).expect(http.StatusForbidden, "owner_protected")
		admin.b.do(http.MethodPatch, path("member"), map[string]string{"role": "admin"}).expect(http.StatusForbidden, "forbidden") // only the Owner manages Admins
		admin.b.do(http.MethodPatch, path("guest"), map[string]string{"role": "owner"}).expect(http.StatusForbidden, "ownership_transfer_required")
		admin.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "boss@acme.test", "role": "admin"}).expect(http.StatusForbidden, "forbidden")
		admin.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "friend@acme.test", "role": "member"}).expect(http.StatusCreated)

		// Deactivation removes access immediately; reactivation restores it.
		admin.b.do(http.MethodPatch, path("guest"), map[string]string{"status": "deactivated"}).expect(http.StatusOK)
		guest.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusNotFound)
		if list := guest.b.refresh()["organisations"].([]any); len(list) != 0 {
			t.Fatalf("deactivated member still lists the organisation: %v", list)
		}
		admin.b.do(http.MethodPatch, path("guest"), map[string]string{"status": "active"}).expect(http.StatusOK)
		guest.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusOK)

		// Role changes apply on the next request.
		ownerA.b.do(http.MethodPatch, path("member"), map[string]string{"role": "admin"}).expect(http.StatusOK)
		member.b.do(http.MethodGet, "/orgs/"+orgA+"/settings/security", nil).expect(http.StatusOK)
		ownerA.b.do(http.MethodPatch, path("member"), map[string]string{"role": "member"}).expect(http.StatusOK)
		member.b.do(http.MethodGet, "/orgs/"+orgA+"/settings/security", nil).expect(http.StatusForbidden)
		if h.auditCount(orgA, "member.role_changed") != 2 || h.auditCount(orgA, "member.status_changed") != 2 {
			t.Fatal("member audit entries missing")
		}
	})

	h.run("security settings: concurrency, invite policy, 2FA and session age", func(t *testing.T) {
		settings := "/orgs/" + orgA + "/settings/security"
		res := ownerA.b.do(http.MethodGet, settings, nil).expect(http.StatusOK)
		etag := res.header.Get("ETag")
		doc := res.m()
		if doc["invite_permission"] != "admins" || doc["session_max_age"] != "30d" || etag == "" {
			t.Fatalf("defaults: %v %q", doc, etag)
		}
		body := map[string]any{"require_2fa": false, "session_max_age": "30d", "allow_guest_invites": true, "invite_permission": "members"}
		ownerA.b.do(http.MethodPut, settings, body).expect(http.StatusPreconditionRequired, "precondition_required")
		ownerA.b.do(http.MethodPut, settings, body, header("If-Match", `"999"`)).expect(http.StatusPreconditionFailed, "version_mismatch")
		ownerA.b.do(http.MethodPut, settings, map[string]any{"require_2fa": false, "session_max_age": "1y", "allow_guest_invites": true, "invite_permission": "everyone"}, header("If-Match", etag)).
			expect(http.StatusUnprocessableEntity)
		res = ownerA.b.do(http.MethodPut, settings, body, header("If-Match", etag)).expect(http.StatusOK)
		ownerA.b.do(http.MethodPut, settings, body, header("If-Match", etag)).expect(http.StatusPreconditionFailed) // stale now
		etag = res.header.Get("ETag")

		// Members may now invite Members, but still not Guests or Admins.
		member.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "peer@acme.test", "role": "member"}).expect(http.StatusCreated)
		member.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "peer-guest@acme.test", "role": "guest"}).expect(http.StatusForbidden)

		// Only an administrator who has 2FA can require it (otherwise they would lock themselves out).
		body = map[string]any{"require_2fa": true, "session_max_age": "12h", "allow_guest_invites": false, "invite_permission": "admins"}
		ownerA.b.do(http.MethodPut, settings, body, header("If-Match", etag)).expect(http.StatusConflict, "mfa_required_for_policy")
		secret := ownerA.b.do(http.MethodPost, "/auth/mfa/totp/setup", nil).expect(http.StatusOK).m()["secret"].(string)
		h.remember(secret)
		ownerA.b.do(http.MethodPost, "/auth/mfa/totp/enable", map[string]string{"code": h.totpCode(secret)}).expect(http.StatusOK)
		res = ownerA.b.do(http.MethodPut, settings, body, header("If-Match", etag)).expect(http.StatusOK)
		etag = res.header.Get("ETag")

		// Requiring 2FA blocks members without it from this organisation only.
		member.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusForbidden, "mfa_enrollment_required")
		member.b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusOK) // account routes still work (to enrol)
		ownerB.b.do(http.MethodGet, "/orgs/"+orgB, nil).expect(http.StatusOK)   // other organisations unaffected
		ownerA.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusOK)   // enrolled
		ownerA.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]string{"email": "g2@acme.test", "role": "guest"}).
			expect(http.StatusForbidden, "forbidden") // guest invitations now off

		// Session age: older than 12 h → sign in again for this organisation only.
		body["require_2fa"] = false
		ownerA.b.do(http.MethodPut, settings, body, header("If-Match", etag)).expect(http.StatusOK)
		member.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusOK)
		h.clock.Advance(13 * time.Hour)
		member.b.do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusUnauthorized, "session_expired")
		member.b.do(http.MethodGet, "/auth/session", nil).expect(http.StatusOK)
		ownerB.b.do(http.MethodGet, "/orgs/"+orgB, nil).expect(http.StatusOK) // Globex keeps 30 d
		h.clock.Advance(-13 * time.Hour)
		if h.auditCount(orgA, "org.security_updated") != 3 {
			t.Fatal("settings audit entries missing")
		}
	})

	h.run("cross-organisation access is indistinguishable from absence", func(t *testing.T) {
		_, bToken := h.invite(ownerB.b, orgB, "staff@globex.test", "member")
		bMember := h.join(bToken, "staff@globex.test", "Globex Staff")
		bMemberID := bMember.b.refresh()["user"].(map[string]any)["id"].(string)
		invB, _ := h.invite(ownerB.b, orgB, "only-b@globex.test", "member")

		// A's admin using B's organisation path.
		for _, p := range []string{"", "/members", "/members/" + bMemberID, "/invitations", "/settings/security"} {
			admin.b.do(http.MethodGet, "/orgs/"+orgB+p, nil).expect(http.StatusNotFound, "not_found")
		}
		admin.b.do(http.MethodPatch, "/orgs/"+orgB, map[string]string{"name": "Taken"}).expect(http.StatusNotFound)
		admin.b.do(http.MethodPost, "/orgs/"+orgB+"/invitations", map[string]string{"email": "spy@globex.test", "role": "admin"}).expect(http.StatusNotFound)
		// A's path with B's resource IDs.
		admin.b.do(http.MethodGet, "/orgs/"+orgA+"/members/"+bMemberID, nil).expect(http.StatusNotFound)
		admin.b.do(http.MethodPatch, "/orgs/"+orgA+"/members/"+bMemberID, map[string]string{"status": "deactivated"}).expect(http.StatusNotFound)
		bMember.b.do(http.MethodGet, "/orgs/"+orgB, nil).expect(http.StatusOK)
		admin.b.do(http.MethodDelete, "/orgs/"+orgA+"/invitations/"+invB, nil).expect(http.StatusNotFound)
		admin.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations/"+invB+"/resend", nil).expect(http.StatusNotFound)
		// Nonexistent and malformed organisation IDs.
		admin.b.do(http.MethodGet, "/orgs/0199a6f0-0000-7000-8000-000000000000", nil).expect(http.StatusNotFound)
		admin.b.do(http.MethodGet, "/orgs/not-a-uuid/members", nil).expect(http.StatusNotFound)
		// Signed out.
		h.browser().do(http.MethodGet, "/orgs/"+orgA, nil).expect(http.StatusUnauthorized, "unauthenticated")
		// B's invitation is still intact.
		var inv struct {
			Data []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		ownerB.b.do(http.MethodGet, "/orgs/"+orgB+"/invitations", nil).expect(http.StatusOK).json(&inv)
		if len(inv.Data) == 0 || inv.Data[0].ID != invB || inv.Data[0].Status != "pending" {
			t.Fatalf("B's invitation changed: %+v", inv)
		}
		if h.auditCount(orgB, "invitation.revoked") != 0 {
			t.Fatal("cross-tenant revoke reached B")
		}
	})

	h.run("organisation profile and self-service creation", func(t *testing.T) {
		ownerB.b.do(http.MethodPatch, "/orgs/"+orgB, map[string]string{"name": "  Globex‮ Corp "}).expect(http.StatusOK)
		if name := ownerB.b.do(http.MethodGet, "/orgs/"+orgB, nil).expect(http.StatusOK).m()["name"]; name != "Globex Corp" {
			t.Fatalf("name not cleaned: %q", name)
		}
		ownerB.b.do(http.MethodPatch, "/orgs/"+orgB, map[string]string{"name": ""}).expect(http.StatusUnprocessableEntity)
		ownerB.b.do(http.MethodPost, "/orgs", map[string]string{"name": "Mine", "slug": "mine"}).expect(http.StatusForbidden, "org_signup_disabled")
	})

	h.run("database isolation for the new tables", func(t *testing.T) {
		// The runtime role sees nothing without a tenant or user context.
		for _, table := range []string{"organisations", "organisation_memberships", "organisation_settings", "invitations", "audit_logs", "users", "user_emails"} {
			var n int
			if err := h.db.App.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
				t.Fatalf("%s: %v", table, err)
			}
			if n != 0 {
				t.Fatalf("%s: %d rows visible without context", table, n)
			}
		}
		// Audit logs are append-only for the runtime role.
		if _, err := h.db.App.Exec(ctx, "DELETE FROM audit_logs"); err == nil {
			t.Fatal("runtime role may delete audit logs")
		}
		// Pre-context lookups are narrow: the email-membership probe only answers for the current org.
		var leaked bool
		if err := h.db.App.QueryRow(ctx, "SELECT mc_org_email_is_member($1, $2)", orgA, member.email).Scan(&leaked); err != nil || leaked {
			t.Fatalf("membership probe answered without org context: %v %v", leaked, err)
		}
		var total int
		if err := h.db.Owner.QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE org_id = $1", orgA).Scan(&total); err != nil || total == 0 {
			t.Fatalf("audit rows: %d %v", total, err)
		}
	})
}
