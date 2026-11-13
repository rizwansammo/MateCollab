//go:build integration

package apiserver

import (
	"net/http"
	"testing"

	"github.com/rizwansammo/MateCollab/services/internal/emails"
)

// HTTP contract, tenant and guest access regression coverage for Phase 3.
// Every response is checked against the generated OpenAPI schema by browser.do.
func TestSpacesChannelsGuestsAndMessagingPolicy(t *testing.T) {
	h := newHarness(t, nil)
	orgA, owner := h.seedOrg("Acme", "acme", "owner-spaces@acme.test")
	orgB, other := h.seedOrg("Globex", "globex", "owner-spaces@globex.test")
	_, memberToken := h.invite(owner.b, orgA, "member-spaces@acme.test", "member")
	member := h.join(memberToken, "member-spaces@acme.test", "Member")

	space := owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces", map[string]any{
		"name": "Engineering", "kind": "team", "visibility": "public",
	}).expect(http.StatusCreated).m()
	spaceID := space["id"].(string)
	if space["joined"] != true || space["viewer_role"] != "space_admin" {
		t.Fatalf("creator access: %v", space)
	}

	owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusOK)
	member.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusOK)
	member.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels", nil).expect(http.StatusForbidden)
	member.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/join", map[string]any{}).expect(http.StatusNoContent)
	member.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels", nil).expect(http.StatusOK)
	chanList := owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels", nil).m()
	all := chanList["data"].([]any)
	if len(all) != 1 || all[0].(map[string]any)["name"] != "general" {
		t.Fatalf("default channel: %v", chanList)
	}
	general := all[0].(map[string]any)["id"].(string)
	owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+general+"/archive", map[string]any{}).
		expect(http.StatusConflict, "default_channel_protected")

	// Invite a Guest with one explicit grant. No general public-Space discovery.
	otherSpace := owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces", map[string]any{
		"name": "Finance", "kind": "project", "visibility": "private",
	}).expect(http.StatusCreated).m()["id"].(string)
	guestEmail := "guest-spaces@acme.test"
	owner.b.do(http.MethodPost, "/orgs/"+orgA+"/invitations", map[string]any{
		"email": guestEmail, "role": "guest", "space_ids": []string{spaceID},
	}).expect(http.StatusCreated)
	guest := h.join(h.lastToken(guestEmail, emails.Invitation), guestEmail, "Guest")
	guestList := guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces", nil).expect(http.StatusOK).m()
	if rows := guestList["data"].([]any); len(rows) != 1 || rows[0].(map[string]any)["id"] != spaceID {
		t.Fatalf("Guest Space grants not enforced: %v", rows)
	}
	guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+otherSpace, nil).expect(http.StatusNotFound)
	guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels", nil).expect(http.StatusOK)
	guest.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces", map[string]any{"name": "Forbidden"}).expect(http.StatusForbidden)
	guest.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels",
		map[string]any{"name": "forbidden"}).expect(http.StatusForbidden)

	secret := owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels",
		map[string]any{"name": "leadership", "visibility": "private"}).expect(http.StatusCreated).m()["id"].(string)
	guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret, nil).expect(http.StatusNotFound)
	guestID := guest.b.refresh()["user"].(map[string]any)["id"].(string)
	owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret+"/members",
		map[string]any{"user_id": guestID}).expect(http.StatusNoContent)
	guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret, nil).expect(http.StatusOK)
	guest.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret+"/members",
		map[string]any{"user_id": guestID}).expect(http.StatusForbidden)

	// Versioned administration policy is enforced at the write boundary.
	policy := owner.b.do(http.MethodGet, "/orgs/"+orgA+"/settings/messaging", nil).expect(http.StatusOK)
	owner.b.do(http.MethodPut, "/orgs/"+orgA+"/settings/messaging",
		map[string]any{"public_space_creation": "admins", "private_space_creation": "admins"},
		header("If-Match", policy.header.Get("ETag"))).expect(http.StatusOK)
	member.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces",
		map[string]any{"name": "Not permitted", "visibility": "public"}).expect(http.StatusForbidden)
	owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces", map[string]any{
		"name": "Owner Space", "visibility": "public",
	}).expect(http.StatusCreated)

	// Access to another organisation and another tenant's resources must remain indistinguishable.
	other.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces", nil).expect(http.StatusNotFound)
	other.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusNotFound)
	owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+general, nil).expect(http.StatusOK)
	owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+otherSpace+"/archive", map[string]any{}).expect(http.StatusNoContent)
	if rows := guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces", nil).m()["data"].([]any); len(rows) != 1 {
		t.Fatalf("guest list changed unexpectedly: %v", rows)
	}
	// Verify the other organisation remains accessible to its Owner after the isolation probes.
	other.b.do(http.MethodGet, "/orgs/"+orgB, nil).expect(http.StatusOK)

	// Full resource lifecycle: versioned updates, membership revocation and cleanup.
	spaceDoc := owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusOK)
	originalETag := spaceDoc.header.Get("ETag")
	body := map[string]any{"name": "Engineering Core", "kind": "team", "visibility": "public", "description": "Main team"}
	owner.b.do(http.MethodPut, "/orgs/"+orgA+"/spaces/"+spaceID, body, header("If-Match", originalETag)).expect(http.StatusOK)
	owner.b.do(http.MethodPut, "/orgs/"+orgA+"/spaces/"+spaceID, body, header("If-Match", originalETag)).expect(http.StatusPreconditionFailed, "version_mismatch")

	channelDoc := owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret, nil).expect(http.StatusOK)
	owner.b.do(http.MethodPut, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret,
		map[string]any{"name": "leadership-notes", "visibility": "private", "topic": "Planning"},
		header("If-Match", channelDoc.header.Get("ETag"))).expect(http.StatusOK)
	owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret+"/members", nil).expect(http.StatusOK)
	owner.b.do(http.MethodDelete, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret+"/members/"+guestID, nil).expect(http.StatusNoContent)
	guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret, nil).expect(http.StatusNotFound)

	memberID := member.b.refresh()["user"].(map[string]any)["id"].(string)
	owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/members", nil).expect(http.StatusOK)
	owner.b.do(http.MethodPut, "/orgs/"+orgA+"/spaces/"+spaceID+"/members/"+memberID,
		map[string]any{"role": "member"}).expect(http.StatusNoContent)
	member.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/leave", map[string]any{}).expect(http.StatusNoContent)
	member.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels", nil).expect(http.StatusForbidden)
	owner.b.do(http.MethodDelete, "/orgs/"+orgA+"/spaces/"+spaceID+"/members/"+guestID, nil).expect(http.StatusNoContent)
	guest.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusNotFound)
	owner.b.do(http.MethodPost, "/orgs/"+orgA+"/spaces/"+spaceID+"/channels/"+secret+"/archive",
		map[string]any{}).expect(http.StatusNoContent)
	owner.b.do(http.MethodDelete, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusNoContent)
	owner.b.do(http.MethodGet, "/orgs/"+orgA+"/spaces/"+spaceID, nil).expect(http.StatusNotFound)
}
