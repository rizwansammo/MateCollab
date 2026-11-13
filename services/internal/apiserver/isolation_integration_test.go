//go:build integration

package apiserver

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestEveryTenantRouteIsIsolated is the generated API suite of TESTING.md (tenant isolation):
// every /orgs/{orgId}/… route is called (1) by a member of organisation B with organisation A's
// ID and (2) by A's Owner with A's ID but B's resource IDs. Both must answer 404. A route with a
// path parameter that has no fixture below fails the test, so new routes cannot skip it.
func TestEveryTenantRouteIsIsolated(t *testing.T) {
	h := newHarness(t, nil)
	orgA, ownerA := h.seedOrg("Acme", "acme", "owner@acme.test")
	orgB, ownerB := h.seedOrg("Globex", "globex", "owner@globex.test")
	invB, tok := h.invite(ownerB.b, orgB, "staff@globex.test", "admin")
	staffB := h.join(tok, "staff@globex.test", "Globex Staff")
	pendingB, _ := h.invite(ownerB.b, orgB, "pending@globex.test", "member")
	_ = invB
	spaceB := ownerB.b.do(http.MethodPost, "/orgs/"+orgB+"/spaces", map[string]any{
		"name": "Operations", "kind": "team", "visibility": "public",
	}).expect(http.StatusCreated).m()["id"].(string)
	channelB := ownerB.b.do(http.MethodPost, "/orgs/"+orgB+"/spaces/"+spaceB+"/channels",
		map[string]any{"name": "operations-chat", "visibility": "private"}).expect(http.StatusCreated).m()["id"].(string)

	// Resource IDs that exist only in organisation B.
	fixtures := map[string]string{
		"spaceId":      spaceB,
		"channelId":    channelB,
		"userId":       userID(staffB),
		"invitationId": pendingB,
		"exportId":     "0199a6f0-0000-7000-8000-00000000e0e0", // exports are per organisation; any ID is foreign to A
	}
	// Bodies valid enough to reach the handler's lookup.
	bodies := map[string]any{
		"PATCH /orgs/{orgId}/members/{userId}":                             map[string]string{"status": "suspended"},
		"POST /orgs/{orgId}/ownership-transfer":                            map[string]string{"user_id": userID(staffB)},
		"PATCH /orgs/{orgId}":                                              map[string]string{"name": "Taken"},
		"PATCH /orgs/{orgId}/me/membership":                                map[string]string{"title": "x"},
		"POST /orgs/{orgId}/invitations":                                   map[string]string{"email": "spy@acme.test", "role": "member"},
		"PUT /orgs/{orgId}/settings/security":                              map[string]any{"require_2fa": false, "session_max_age": "30d", "allow_guest_invites": true, "invite_permission": "admins"},
		"POST /orgs/{orgId}/audit-logs/exports":                            map[string]any{},
		"POST /orgs/{orgId}/spaces/{spaceId}/channels":                     map[string]any{"name": "alpha"},
		"PUT /orgs/{orgId}/spaces/{spaceId}":                               map[string]any{"name": "alpha"},
		"PUT /orgs/{orgId}/spaces/{spaceId}/members/{userId}":              map[string]any{"role": "member"},
		"PUT /orgs/{orgId}/spaces/{spaceId}/channels/{channelId}":          map[string]any{"name": "alpha"},
		"POST /orgs/{orgId}/spaces/{spaceId}/channels/{channelId}/members": map[string]any{"user_id": userID(staffB)},
	}

	type route struct{ method, pattern string }
	var routes []route
	err := chi.Walk(h.routes, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		pattern = strings.TrimSuffix(strings.TrimPrefix(pattern, "/api/v1"), "/")
		if strings.HasPrefix(pattern, "/orgs/{orgId}") {
			routes = append(routes, route{method, pattern})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].pattern+routes[i].method < routes[j].pattern+routes[j].method })
	if len(routes) < 15 {
		t.Fatalf("only %d tenant routes found", len(routes))
	}

	fill := func(pattern, org string) string {
		path := strings.ReplaceAll(pattern, "{orgId}", org)
		for {
			i := strings.Index(path, "{")
			if i < 0 {
				return path
			}
			j := strings.Index(path, "}")
			name := path[i+1 : j]
			id, ok := fixtures[name]
			if !ok {
				t.Fatalf("no cross-tenant fixture for {%s} in %s", name, pattern)
			}
			path = path[:i] + id + path[j+1:]
		}
	}

	for _, rt := range routes {
		key := rt.method + " " + rt.pattern
		body := bodies[key]
		if body == nil && rt.method != http.MethodGet && rt.method != http.MethodDelete {
			body = map[string]any{}
		}
		// PUT uses a resource version; supply a valid header to reach resource lookup.
		var opts []reqOpt
		if rt.method == http.MethodPut {
			opts = append(opts, header("If-Match", `"1"`))
		}
		// (1) A member of B addresses organisation A.
		res := ownerB.b.do(rt.method, fill(rt.pattern, orgA), body, opts...)
		if res.status != http.StatusNotFound {
			t.Errorf("%s as a non-member: %d %s", key, res.status, res.body)
		}
		// (2) A's Owner addresses B's resources through A's path.
		if strings.Count(rt.pattern, "{") > 1 {
			res := ownerA.b.do(rt.method, fill(rt.pattern, orgA), body, opts...)
			if res.status != http.StatusNotFound {
				t.Errorf("%s with B's resource IDs: %d %s", key, res.status, res.body)
			}
		}
	}
	// B's data is untouched.
	staffB.b.do(http.MethodGet, "/orgs/"+orgB, nil).expect(http.StatusOK)
	ownerB.b.do(http.MethodGet, "/orgs/"+orgB+"/invitations?status=pending", nil).expect(http.StatusOK)
}
