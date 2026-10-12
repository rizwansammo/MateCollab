// Package orgcontext turns /api/v1/orgs/{orgId}/… requests into an organisation Actor
// (MULTITENANCY.md §2, layers 1–2). It runs after session authentication and before every
// tenant handler, so handlers never see a request whose membership was not checked.
package orgcontext

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/memberships"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Resolver supplies membership resolution and the organisation's security policy.
type Resolver interface {
	Resolve(ctx context.Context, user tenant.UserID, org tenant.OrgID) (memberships.Resolved, error)
}

// SecurityPolicy loads an organisation's security settings.
type SecurityPolicy interface {
	SecurityFor(ctx context.Context, org tenant.OrgID) (orgs.Security, error)
}

var (
	errOrgSuspended        = apperr.Forbidden("org_suspended", "This organisation is suspended. Contact its owner.")
	errMembershipSuspended = apperr.Forbidden("membership_suspended", "Your access to this organisation is suspended.")
	errMFAEnrollment       = apperr.Forbidden("mfa_enrollment_required", "This organisation requires two-step verification. Turn it on in your security settings to continue.")
	errSessionTooOld       = apperr.Unauthenticated("session_expired", "This organisation requires you to sign in again.")
)

// Middleware resolves the {orgId} path parameter for the signed-in user. Unknown
// organisations, organisations the user does not belong to and deactivated memberships are all
// reported as 404, so membership elsewhere cannot be probed.
func Middleware(members Resolver, policy SecurityPolicy, now func() time.Time) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteProblem(w, r, auth.ErrNoSession)
				return
			}
			id, err := uuid.Parse(chi.URLParam(r, "orgId"))
			if err != nil {
				httpx.WriteProblem(w, r, apperr.NotFound())
				return
			}
			org := tenant.OrgID(id)
			m, err := members.Resolve(r.Context(), p.UserID, org)
			if err != nil {
				httpx.WriteProblem(w, r, err)
				return
			}
			switch {
			case m.Status == "deactivated" || m.OrgStatus == "deleting":
				httpx.WriteProblem(w, r, apperr.NotFound())
				return
			case m.OrgStatus == "suspended":
				httpx.WriteProblem(w, r, errOrgSuspended)
				return
			case m.Status != "active":
				httpx.WriteProblem(w, r, errMembershipSuspended)
				return
			}
			sec, err := policy.SecurityFor(r.Context(), org)
			if err != nil {
				httpx.WriteProblem(w, r, err)
				return
			}
			if now().Sub(p.CreatedAt) > sec.MaxAge() {
				httpx.WriteProblem(w, r, errSessionTooOld)
				return
			}
			if sec.Require2FA && !p.TOTPEnabled {
				httpx.WriteProblem(w, r, errMFAEnrollment)
				return
			}
			ctx := tenant.WithActor(r.Context(), tenant.Actor{OrgID: org, UserID: p.UserID, Role: m.Role, SessionID: p.SessionID})
			ctx = authz.WithSettings(ctx, sec.Authz())
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
