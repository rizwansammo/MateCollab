// Package apiserver assembles the public HTTP router of the API binary. Feature modules mount
// their routes here as they are implemented; the middleware order follows BACKEND.md §3.
package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/auth"
	"github.com/rizwansammo/MateCollab/services/internal/clientconfig"
	"github.com/rizwansammo/MateCollab/services/internal/memberships"
	"github.com/rizwansammo/MateCollab/services/internal/orgcontext"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/idempotency"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/users"
)

// Router returns the /api/v1 router with the platform middleware chain. When svc is nil only
// the platform routes are mounted (unit tests of the transport layer).
func Router(cfg *config.Config, logger *slog.Logger, m *telemetry.Metrics, svc *Services) http.Handler {
	r := chi.NewRouter()
	r.Use(
		httpx.RequestID,
		httpx.RealIP(cfg.TrustedProxies),
		httpx.Observe(logger, m),
		httpx.Recover,
		httpx.APISecurityHeaders,
		httpx.BodyLimit(httpx.DefaultBodyLimit),
	)
	r.NotFound(httpx.NotFoundHandler)
	r.MethodNotAllowed(httpx.MethodNotAllowedHandler)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/config", clientconfig.Handler(cfg))
		if svc != nil {
			mountModules(r, cfg, svc)
		}
	})
	return r
}

// mountModules wires the feature modules: session resolution, then CSRF, then routes. Tenant
// routes live under /orgs/{orgId} behind the organisation context middleware, which resolves
// membership before any handler runs (MULTITENANCY.md §2).
func mountModules(r chi.Router, cfg *config.Config, svc *Services) {
	authH := auth.NewHandler(svc.Auth, svc.SessionView)
	usersH := users.NewHandler(svc.Users)
	idem := &idempotency.Store{Pool: svc.Pool, Now: svc.Auth.Now, User: func(r *http.Request) (uuid.UUID, bool) {
		p, ok := auth.PrincipalFrom(r.Context())
		return uuid.UUID(p.UserID), ok
	}}
	membersH := memberships.NewHandler(svc.Memberships, idem.Middleware)
	orgsH := orgs.NewHandler(svc.Orgs)
	auditH := audit.NewHandler(svc.Audit, func(r *http.Request) string {
		a, _ := tenant.ActorFrom(r.Context())
		if m, err := svc.Memberships.Resolve(r.Context(), a.UserID, a.OrgID); err == nil {
			return m.Slug
		}
		return "organisation"
	})

	r.Group(func(r chi.Router) {
		r.Use(svc.Auth.Middleware, svc.Auth.CSRF(cfg.AllowedOrigins))
		authH.Routes(r)
		membersH.PublicRoutes(r)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireSession)
			usersH.Routes(r)
			membersH.SessionRoutes(r)
			orgsH.SessionRoutes(r)
			r.Route("/orgs/{orgId}", func(r chi.Router) {
				r.Use(orgcontext.Middleware(svc.Memberships, svc.Orgs, svc.Auth.Now))
				orgsH.OrgRoutes(r)
				membersH.OrgRoutes(r)
				auditH.OrgRoutes(r)
			})
		})
	})
}
