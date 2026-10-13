//go:build integration

package apiserver

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/idempotency"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
	"github.com/rizwansammo/MateCollab/services/internal/users"
)

func userID(a *account) string {
	return a.b.refresh()["user"].(map[string]any)["id"].(string)
}

func TestOwnershipTransfer(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	orgA, owner := h.seedOrg("Acme", "acme", "owner@acme.test")
	_, ownerB := h.seedOrg("Globex", "globex", "owner@globex.test")
	_, tok := h.invite(owner.b, orgA, "admin@acme.test", "admin")
	admin := h.join(tok, "admin@acme.test", "Ada Admin")
	_, tok = h.invite(owner.b, orgA, "admin2@acme.test", "admin")
	admin2 := h.join(tok, "admin2@acme.test", "Abe Admin")
	_, tok = h.invite(owner.b, orgA, "member@acme.test", "member")
	member := h.join(tok, "member@acme.test", "Mo Member")
	_, tok = h.invite(owner.b, orgA, "guest@acme.test", "guest")
	guest := h.join(tok, "guest@acme.test", "Gus Guest")
	path := "/orgs/" + orgA + "/ownership-transfer"
	owners := func() int {
		var n int
		if err := h.db.Owner.QueryRow(ctx, `SELECT count(*) FROM organisation_memberships WHERE org_id = $1 AND role = 'owner'`, orgA).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	h.run("only the Owner, after a recent password check, may transfer", func(t *testing.T) {
		admin.b.do(http.MethodPost, path, map[string]string{"user_id": userID(admin2)}).expect(http.StatusForbidden, "forbidden")
		h.clock.Advance(auth.StepUpWindow + time.Minute)
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(admin)}).expect(http.StatusForbidden, "reauthentication_required")
		h.clock.Advance(-(auth.StepUpWindow + time.Minute))
		ownerB.b.do(http.MethodPost, path, map[string]string{"user_id": userID(admin)}).expect(http.StatusNotFound) // other tenant
	})

	h.run("invalid targets are refused", func(t *testing.T) {
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(owner)}).expect(http.StatusConflict, "transfer_to_self")
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(member)}).expect(http.StatusConflict, "transfer_target_not_admin")
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(guest)}).expect(http.StatusConflict, "transfer_target_not_admin")
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(ownerB)}).expect(http.StatusNotFound) // not a member here
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": "0199a6f0-0000-7000-8000-000000000000"}).expect(http.StatusNotFound)
		owner.b.do(http.MethodPost, path, map[string]any{}).expect(http.StatusUnprocessableEntity)
		owner.b.do(http.MethodPatch, "/orgs/"+orgA+"/members/"+userID(admin2), map[string]string{"status": "deactivated"}).expect(http.StatusOK)
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(admin2)}).expect(http.StatusConflict, "transfer_target_inactive")
		if owners() != 1 {
			t.Fatal("ownership changed after a refused transfer")
		}
	})

	h.run("transfer swaps the roles atomically and is audited", func(t *testing.T) {
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(admin)}).expect(http.StatusNoContent)
		var me, them struct {
			Role string `json:"role"`
		}
		owner.b.do(http.MethodGet, "/orgs/"+orgA+"/members/"+userID(owner), nil).expect(http.StatusOK).json(&me)
		owner.b.do(http.MethodGet, "/orgs/"+orgA+"/members/"+userID(admin), nil).expect(http.StatusOK).json(&them)
		if me.Role != "admin" || them.Role != "owner" || owners() != 1 {
			t.Fatalf("after transfer: previous=%s new=%s owners=%d", me.Role, them.Role, owners())
		}
		if h.auditCount(orgA, "org.ownership_transferred") != 1 {
			t.Fatal("transfer not audited")
		}
		// The previous Owner lost the right immediately; the new Owner holds it.
		owner.b.do(http.MethodPost, path, map[string]string{"user_id": userID(admin)}).expect(http.StatusForbidden)
		admin.b.do(http.MethodGet, "/orgs/"+orgA+"/settings/security", nil).expect(http.StatusOK)
	})

	h.run("operator assignment (mcctl user create-owner)", func(t *testing.T) {
		res, err := h.svc.Orgs.AssignOwner(ctx, "ACME", orgs.OwnerSpec{Email: "rescue@acme.test", DisplayName: "Rescue Owner"})
		if err != nil {
			t.Fatal(err)
		}
		if !res.NewAccount || res.SetupLink == "" || res.PreviousOwner.String() != userID(admin) || owners() != 1 {
			t.Fatalf("assign: %+v owners=%d", res, owners())
		}
		h.remember(res.SetupLink)
		again, err := h.svc.Orgs.AssignOwner(ctx, "acme", orgs.OwnerSpec{Email: "rescue@acme.test"})
		if err != nil || again.PreviousOwner != again.OwnerID || again.NewAccount {
			t.Fatalf("second assign should be a no-op: %+v %v", again, err)
		}
		if _, err := h.svc.Orgs.AssignOwner(ctx, "nope", orgs.OwnerSpec{Email: "x@acme.test"}); err == nil {
			t.Fatal("unknown organisation accepted")
		}
		// An existing member is promoted; the previous Owner becomes an Admin.
		res, err = h.svc.Orgs.AssignOwner(ctx, "acme", orgs.OwnerSpec{Email: owner.email})
		if err != nil || res.NewAccount || owners() != 1 {
			t.Fatalf("promote member: %+v %v owners=%d", res, err, owners())
		}
	})
}

func TestOwnMembershipProfile(t *testing.T) {
	h := newHarness(t, nil)
	orgA, owner := h.seedOrg("Acme", "acme", "owner@acme.test")
	orgB, _ := h.seedOrg("Globex", "globex", "owner@globex.test")
	_, tok := h.invite(owner.b, orgA, "member@acme.test", "member")
	member := h.join(tok, "member@acme.test", "Mo Member")
	path := "/orgs/" + orgA + "/me/membership"

	var m struct {
		Role   string  `json:"role"`
		Status string  `json:"status"`
		Title  *string `json:"title"`
		Email  *string `json:"email"`
	}
	member.b.do(http.MethodPatch, path, map[string]any{"title": "  Head of‮ Ops "}).expect(http.StatusOK).json(&m)
	if m.Title == nil || *m.Title != "Head of Ops" || m.Email == nil || *m.Email != "member@acme.test" {
		t.Fatalf("title: %+v", m)
	}
	// Role, status and organisation cannot be changed here.
	member.b.do(http.MethodPatch, path, map[string]any{"role": "owner"}).expect(http.StatusBadRequest)
	member.b.do(http.MethodPatch, path, map[string]any{"status": "active"}).expect(http.StatusBadRequest)
	member.b.do(http.MethodPatch, path, map[string]any{"title": strings.Repeat("x", 101)}).expect(http.StatusUnprocessableEntity)

	// Contact email: only the member's own confirmed addresses.
	member.b.do(http.MethodPatch, path, map[string]any{"contact_email": "owner@acme.test"}).expect(http.StatusUnprocessableEntity, "validation_failed")
	member.b.do(http.MethodPost, "/me/emails", map[string]string{"email": "mo@personal.test"}).expect(http.StatusCreated)
	member.b.do(http.MethodPatch, path, map[string]any{"contact_email": "mo@personal.test"}).expect(http.StatusUnprocessableEntity) // unconfirmed
	h.browser().do(http.MethodPost, "/auth/email/verify", map[string]string{"token": h.lastToken("mo@personal.test", "email_verification")}).expect(http.StatusNoContent)
	member.b.do(http.MethodPatch, path, map[string]any{"contact_email": "mo@personal.test", "title": nil}).expect(http.StatusOK).json(&m)
	if m.Email == nil || *m.Email != "mo@personal.test" || m.Title != nil || m.Role != "member" || m.Status != "active" {
		t.Fatalf("contact email: %+v", m)
	}
	if h.auditCount(orgA, "member.profile_updated") != 2 {
		t.Fatal("profile changes not audited")
	}
	// Not a member of the other organisation.
	member.b.do(http.MethodPatch, "/orgs/"+orgB+"/me/membership", map[string]any{"title": "x"}).expect(http.StatusNotFound)
}

func TestAccountDeletion(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	orgA, owner := h.seedOrg("Acme", "acme", "owner@acme.test")
	_, tok := h.invite(owner.b, orgA, "member@acme.test", "member")
	member := h.join(tok, "member@acme.test", "Mo Member")
	_, tok = h.invite(owner.b, orgA, "admin@acme.test", "admin")
	admin := h.join(tok, "admin@acme.test", "Ada Admin")
	memberID := userID(member)

	h.run("owners must transfer ownership first", func(t *testing.T) {
		owner.b.do(http.MethodPost, "/me/deletion", nil).expect(http.StatusConflict, "ownership_transfer_required")
	})

	h.run("request needs a recent password check, then schedules a 30-day grace period", func(t *testing.T) {
		h.clock.Advance(auth.StepUpWindow + time.Minute)
		member.b.do(http.MethodPost, "/me/deletion", nil).expect(http.StatusForbidden, "reauthentication_required")
		member.b.do(http.MethodPost, "/auth/reauthenticate", map[string]string{"password": member.password}).expect(http.StatusNoContent)
		h.clock.Advance(-(auth.StepUpWindow + time.Minute))
		res := member.b.do(http.MethodPost, "/me/deletion", nil).expect(http.StatusAccepted).m()
		at, err := time.Parse(time.RFC3339Nano, res["scheduled_at"].(string))
		if err != nil || at.Sub(time.Now()) < auth.DeletionGrace-time.Hour {
			t.Fatalf("scheduled_at %v (%v)", res, err)
		}
		if member.b.refresh()["user"].(map[string]any)["deletion_scheduled_at"] == nil {
			t.Fatal("session document does not show the pending deletion")
		}
		member.b.do(http.MethodPost, "/me/deletion", nil).expect(http.StatusConflict, "deletion_already_requested")
		if n := len(h.emails("member@acme.test")); n == 0 || h.emails("member@acme.test")[n-1].Template != "account_deletion_requested" {
			t.Fatal("no deletion email")
		}
	})

	h.run("a user scheduled for deletion cannot receive ownership", func(t *testing.T) {
		owner.b.do(http.MethodPatch, "/orgs/"+orgA+"/members/"+memberID, map[string]string{"role": "admin"}).expect(http.StatusOK)
		owner.b.do(http.MethodPost, "/orgs/"+orgA+"/ownership-transfer", map[string]string{"user_id": memberID}).expect(http.StatusConflict, "transfer_target_inactive")
	})

	h.run("cancel", func(t *testing.T) {
		member.b.do(http.MethodDelete, "/me/deletion", nil).expect(http.StatusNoContent)
		member.b.do(http.MethodDelete, "/me/deletion", nil).expect(http.StatusConflict, "no_deletion_requested")
		if member.b.refresh()["user"].(map[string]any)["deletion_scheduled_at"] != nil {
			t.Fatal("deletion still pending")
		}
		var n int
		if err := h.db.Owner.QueryRow(ctx, `SELECT count(*) FROM account_audit_events WHERE user_id = $1 AND action IN ('account.deletion_requested','account.deletion_cancelled')`, memberID).Scan(&n); err != nil || n != 2 {
			t.Fatalf("account events: %d %v", n, err)
		}
	})

	h.run("after the grace period the account is anonymised, never an Owner", func(t *testing.T) {
		member.b.do(http.MethodPost, "/me/deletion", nil).expect(http.StatusAccepted)
		admin.b.do(http.MethodPost, "/me/deletion", nil).expect(http.StatusAccepted)
		// The admin becomes Owner through operator tooling after requesting deletion: the purge
		// must refuse to remove an Owner.
		if _, err := h.svc.Orgs.AssignOwner(ctx, "acme", orgs.OwnerSpec{Email: "admin@acme.test"}); err != nil {
			t.Fatal(err)
		}
		worker := &users.PurgeWorker{Pool: h.db.App, Logger: testkit.Logger(t), Now: func() time.Time { return time.Now().Add(auth.DeletionGrace + time.Hour) }}
		if err := worker.Work(ctx, nil); err != nil {
			t.Fatal(err)
		}
		status := func(id string) (s string, name string) {
			if err := h.db.Owner.QueryRow(ctx, `SELECT status, display_name FROM users WHERE id = $1`, id).Scan(&s, &name); err != nil {
				t.Fatal(err)
			}
			return
		}
		if s, name := status(memberID); s != "anonymised" || name != "Deleted user" {
			t.Fatalf("member: %s %s", s, name)
		}
		if s, _ := status(userID(admin)); s != "active" {
			t.Fatalf("an Owner was anonymised: %s", s)
		}
		var emails, sessions int
		var memberStatus string
		_ = h.db.Owner.QueryRow(ctx, `SELECT count(*) FROM user_emails WHERE user_id = $1`, memberID).Scan(&emails)
		_ = h.db.Owner.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id = $1`, memberID).Scan(&sessions)
		_ = h.db.Owner.QueryRow(ctx, `SELECT status FROM organisation_memberships WHERE user_id = $1`, memberID).Scan(&memberStatus)
		if emails != 0 || sessions != 0 || memberStatus != "deactivated" {
			t.Fatalf("leftovers: emails=%d sessions=%d membership=%s", emails, sessions, memberStatus)
		}
		h.browser().do(http.MethodPost, "/auth/login", map[string]string{"email": "member@acme.test", "password": member.password}).expect(http.StatusUnauthorized)
	})
}

func TestOrganisationGeneralAndRoles(t *testing.T) {
	h := newHarness(t, nil)
	orgA, owner := h.seedOrg("Acme", "acme", "owner@acme.test")
	_, tok := h.invite(owner.b, orgA, "member@acme.test", "member")
	member := h.join(tok, "member@acme.test", "Mo Member")
	_, tok = h.invite(owner.b, orgA, "guest@acme.test", "guest")
	guest := h.join(tok, "guest@acme.test", "Gus Guest")
	path := "/orgs/" + orgA

	h.run("profile updates with optimistic concurrency", func(t *testing.T) {
		etag := owner.b.do(http.MethodGet, path, nil).expect(http.StatusOK).header.Get("ETag")
		body := map[string]string{"name": "Acme Corporation", "default_timezone": "Asia/Dhaka", "default_locale": "en-gb"}
		res := owner.b.do(http.MethodPatch, path, body, header("If-Match", etag)).expect(http.StatusOK)
		org := res.m()
		if org["name"] != "Acme Corporation" || org["default_timezone"] != "Asia/Dhaka" || org["default_locale"] != "en-GB" {
			t.Fatalf("profile: %v", org)
		}
		owner.b.do(http.MethodPatch, path, map[string]string{"name": "Stale"}, header("If-Match", etag)).expect(http.StatusPreconditionFailed, "version_mismatch")
		owner.b.do(http.MethodPatch, path, map[string]string{"default_timezone": "Mars/Base"}).expect(http.StatusUnprocessableEntity)
		owner.b.do(http.MethodPatch, path, map[string]string{"default_locale": "not a locale"}).expect(http.StatusUnprocessableEntity)
		member.b.do(http.MethodPatch, path, map[string]string{"name": "Mine"}).expect(http.StatusForbidden)
		// The new name reaches every member's organisation list immediately.
		if name := member.b.refresh()["organisations"].([]any)[0].(map[string]any)["name"]; name != "Acme Corporation" {
			t.Fatalf("member sees %v", name)
		}
		if h.auditCount(orgA, "org.updated") != 1 {
			t.Fatal("profile change not audited")
		}
	})

	h.run("roles and effective permissions come from the authz catalogue", func(t *testing.T) {
		var doc struct {
			Permissions []struct {
				ID string `json:"id"`
			} `json:"permissions"`
			Roles []struct {
				Role        string   `json:"role"`
				Permissions []string `json:"permissions"`
			} `json:"roles"`
		}
		guest.b.do(http.MethodGet, path+"/roles", nil).expect(http.StatusOK).json(&doc)
		has := func(role, perm string) bool {
			for _, r := range doc.Roles {
				if r.Role == role {
					for _, p := range r.Permissions {
						if p == perm {
							return true
						}
					}
				}
			}
			return false
		}
		if len(doc.Permissions) < 10 || !has("owner", "org.ownership.transfer") || has("admin", "org.ownership.transfer") ||
			!has("member", "org.directory.view") || has("guest", "org.directory.view") || has("member", "org.members.invite") {
			t.Fatalf("catalogue: %+v", doc)
		}
		settings := owner.b.do(http.MethodGet, path+"/settings/security", nil).expect(http.StatusOK)
		owner.b.do(http.MethodPut, path+"/settings/security",
			map[string]any{"require_2fa": false, "session_max_age": "30d", "allow_guest_invites": true, "invite_permission": "members"},
			header("If-Match", settings.header.Get("ETag"))).expect(http.StatusOK)
		member.b.do(http.MethodGet, path+"/roles", nil).expect(http.StatusOK).json(&doc)
		if !has("member", "org.members.invite") {
			t.Fatal("effective permissions ignore the organisation's settings")
		}
	})
}

func TestInvitationIdempotency(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	orgA, owner := h.seedOrg("Acme", "acme", "owner@acme.test")
	_, tok := h.invite(owner.b, orgA, "admin@acme.test", "admin")
	admin := h.join(tok, "admin@acme.test", "Ada Admin")
	path := "/orgs/" + orgA + "/invitations"
	key := func(k string) reqOpt { return header("Idempotency-Key", k) }
	count := func(email string) int {
		var n int
		if err := h.db.Owner.QueryRow(ctx, `SELECT count(*) FROM invitations WHERE org_id = $1 AND email = $2`, orgA, email).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	first := owner.b.do(http.MethodPost, path, map[string]string{"email": "new@acme.test", "role": "member"}, key("k-1")).expect(http.StatusCreated)
	retry := owner.b.do(http.MethodPost, path, map[string]string{"email": "new@acme.test", "role": "member"}, key("k-1")).expect(http.StatusCreated)
	if first.m()["id"] != retry.m()["id"] || retry.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("retry was not a replay: %s / %s", first.body, retry.body)
	}
	if count("new@acme.test") != 1 || len(h.emails("new@acme.test")) != 1 {
		t.Fatal("a retry created a second invitation or email")
	}
	// The same key with a different request fails safely.
	owner.b.do(http.MethodPost, path, map[string]string{"email": "other@acme.test", "role": "member"}, key("k-1")).expect(http.StatusUnprocessableEntity, "idempotency_key_reused")
	if count("other@acme.test") != 0 {
		t.Fatal("conflicting request was executed")
	}
	// Keys are scoped per user.
	admin.b.do(http.MethodPost, path, map[string]string{"email": "admins@acme.test", "role": "member"}, key("k-1")).expect(http.StatusCreated)
	// Pending-invitation protection still applies without a key, and errors replay too.
	owner.b.do(http.MethodPost, path, map[string]string{"email": "new@acme.test", "role": "member"}).expect(http.StatusConflict, "invitation_pending")
	owner.b.do(http.MethodPost, path, map[string]string{"email": "new@acme.test", "role": "member"}, key("k-2")).expect(http.StatusConflict, "invitation_pending")
	owner.b.do(http.MethodPost, path, map[string]string{"email": "new@acme.test", "role": "member"}, key("k-2")).expect(http.StatusConflict, "invitation_pending")
	// A request still in progress is not run twice.
	ownerID := userID(owner)
	busy := map[string]string{"email": "busy@acme.test", "role": "member"}
	raw, _ := json.Marshal(busy)
	sum := sha256.Sum256(append([]byte("POST\n/api/v1"+path+"\n"), raw...))
	if _, err := h.db.Owner.Exec(ctx, `INSERT INTO idempotency_keys (user_id, key, method, path, request_hash, state, expires_at)
		VALUES ($1, 'k-busy', 'POST', $2, $3, 'in_progress', now() + interval '1 hour')`, ownerID, "/api/v1"+path, sum[:]); err != nil {
		t.Fatal(err)
	}
	owner.b.do(http.MethodPost, path, busy, key("k-busy")).expect(http.StatusConflict, "idempotency_conflict")
	owner.b.do(http.MethodPost, path, map[string]string{"email": "x@acme.test", "role": "member"}, key("not valid")).expect(http.StatusUnprocessableEntity)
	// After 24 hours the key can be used again.
	h.clock.Advance(25 * time.Hour)
	owner.b.login(owner.email, owner.password) // the session expired on the test clock
	owner.b.do(http.MethodPost, path, map[string]string{"email": "later@acme.test", "role": "member"}, key("k-1")).expect(http.StatusCreated)
	h.clock.Advance(-25 * time.Hour)
	// The hourly cleanup (no user context) removes expired keys of every user.
	if _, err := h.db.Owner.Exec(ctx, `UPDATE idempotency_keys SET expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	if err := (&idempotency.CleanupWorker{Pool: h.db.App}).Work(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := h.db.Owner.QueryRow(ctx, `SELECT count(*) FROM idempotency_keys`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("expired keys left: %d %v", left, err)
	}
}
