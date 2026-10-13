//go:build integration

package apiserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
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
