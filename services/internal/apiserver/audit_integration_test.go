//go:build integration

package apiserver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

func TestAuditLog(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	orgA, owner := h.seedOrg("Acme", "acme", "owner@acme.test")
	orgB, ownerB := h.seedOrg("Globex", "globex", "owner@globex.test")
	_, tok := h.invite(owner.b, orgA, "member@acme.test", "member")
	member := h.join(tok, "member@acme.test", "=HYPERLINK(\"http://evil.example\")")
	_, tok = h.invite(owner.b, orgA, "admin@acme.test", "admin")
	admin := h.join(tok, "admin@acme.test", "Ada Admin")
	owner.b.do(http.MethodPatch, "/orgs/"+orgA+"/members/"+userID(member), map[string]string{"role": "guest"}).expect(http.StatusOK)
	h.invite(ownerB.b, orgB, "only-b@globex.test", "member")
	path := "/orgs/" + orgA + "/audit-logs"

	type entry struct {
		Action string `json:"action"`
		Actor  struct {
			UserID      *string `json:"user_id"`
			DisplayName *string `json:"display_name"`
		} `json:"actor"`
		TargetID *string `json:"target_id"`
	}
	type list struct {
		Data []entry `json:"data"`
		Page struct {
			NextCursor *string `json:"next_cursor"`
		} `json:"page"`
	}

	h.run("admins list and filter; others are refused", func(t *testing.T) {
		var all list
		admin.b.do(http.MethodGet, path, nil).expect(http.StatusOK).json(&all)
		if len(all.Data) < 5 || all.Data[0].Action != "member.role_changed" {
			t.Fatalf("log: %+v", all.Data)
		}
		var group list
		owner.b.do(http.MethodGet, path+"?action=invitation", nil).expect(http.StatusOK).json(&group)
		for _, e := range group.Data {
			if !strings.HasPrefix(e.Action, "invitation.") {
				t.Fatalf("prefix filter leaked %s", e.Action)
			}
		}
		var byActor list
		owner.b.do(http.MethodGet, path+"?actor="+userID(owner)+"&action=member.role_changed", nil).expect(http.StatusOK).json(&byActor)
		if len(byActor.Data) != 1 || *byActor.Data[0].TargetID != userID(member) || *byActor.Data[0].Actor.DisplayName == "" {
			t.Fatalf("actor filter: %+v", byActor.Data)
		}
		var empty list
		future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		owner.b.do(http.MethodGet, path+"?from="+future, nil).expect(http.StatusOK).json(&empty)
		if len(empty.Data) != 0 {
			t.Fatal("time filter ignored")
		}
		owner.b.do(http.MethodGet, path+"?from=yesterday", nil).expect(http.StatusUnprocessableEntity)
		owner.b.do(http.MethodGet, path+"?actor=nope", nil).expect(http.StatusUnprocessableEntity)
		member.b.do(http.MethodGet, path, nil).expect(http.StatusForbidden)
		ownerB.b.do(http.MethodGet, path, nil).expect(http.StatusNotFound)
		member.b.do(http.MethodPost, path+"/exports", map[string]any{}).expect(http.StatusForbidden)
	})

	h.run("pagination is keyset-based and bound to the filters", func(t *testing.T) {
		seen := map[string]bool{}
		cur := ""
		for i := 0; i < 20; i++ {
			var page list
			p := path + "?limit=2"
			if cur != "" {
				p += "&cursor=" + cur
			}
			owner.b.do(http.MethodGet, p, nil).expect(http.StatusOK).json(&page)
			if page.Page.NextCursor == nil {
				break
			}
			cur = *page.Page.NextCursor
			seen[cur] = true
		}
		if len(seen) < 2 {
			t.Fatal("did not page")
		}
		owner.b.do(http.MethodGet, path+"?limit=2&action=member&cursor="+cur, nil).expect(http.StatusBadRequest, "invalid_cursor")
	})

	h.run("no secrets reach audit metadata", func(t *testing.T) {
		rows, err := h.db.Owner.Query(ctx, `SELECT metadata::text FROM audit_logs`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var md string
			if err := rows.Scan(&md); err != nil {
				t.Fatal(err)
			}
			for _, s := range h.secrets {
				if s != "" && strings.Contains(md, s) {
					t.Fatalf("audit metadata contains a secret: %.6s…", s)
				}
			}
		}
	})

	h.run("CSV export", func(t *testing.T) {
		if h.storage == nil {
			owner.b.do(http.MethodPost, path+"/exports", map[string]any{}).expect(http.StatusServiceUnavailable, "exports_unavailable")
			t.Skip("object storage not configured (MC_TEST_S3_ENDPOINT)")
		}
		res := owner.b.do(http.MethodPost, path+"/exports", map[string]any{"action": "invitation"}).expect(http.StatusAccepted).m()
		id := res["id"].(string)
		if res["status"] != "pending" {
			t.Fatalf("export: %v", res)
		}
		owner.b.do(http.MethodGet, path+"/exports/"+id+"/download", nil).expect(http.StatusConflict, "export_not_ready")
		ownerB.b.do(http.MethodGet, "/orgs/"+orgB+"/audit-logs/exports/"+id, nil).expect(http.StatusNotFound)

		worker := &audit.ExportWorker{Pool: h.db.App, Storage: h.storage, Jobs: h.queue, Logger: testkit.Logger(t)}
		var args audit.ExportArgs
		args.OrgID, args.ExportID = mustUUID(t, orgA), mustUUID(t, id)
		if err := worker.Work(ctx, &river.Job[audit.ExportArgs]{JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}, Args: args}); err != nil {
			t.Fatal(err)
		}
		done := owner.b.do(http.MethodGet, path+"/exports/"+id, nil).expect(http.StatusOK).m()
		if done["status"] != "completed" || done["row_count"].(float64) < 2 {
			t.Fatalf("export: %v", done)
		}
		owner.b.c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		defer func() { owner.b.c.CheckRedirect = nil }()
		redirect := owner.b.do(http.MethodGet, path+"/exports/"+id+"/download", nil).expect(http.StatusFound)
		resp, err := http.Get(redirect.header.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		csvText := string(body)
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(csvText, "time,actor,") || !strings.Contains(csvText, "invitation.accepted") ||
			strings.Contains(csvText, "member.role_changed") {
			t.Fatalf("csv (%d): %s", resp.StatusCode, csvText)
		}
		if !strings.Contains(csvText, `'=HYPERLINK`) {
			t.Fatalf("formula not neutralised: %s", csvText)
		}
		if h.auditCount(orgA, "audit.export_requested") != 1 || h.auditCount(orgA, "audit.export_downloaded") != 1 {
			t.Fatal("export not audited")
		}
	})
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
