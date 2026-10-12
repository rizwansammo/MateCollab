// Package apiserver assembles the public HTTP router of the API binary. Feature modules mount
// their routes here as they are implemented; the middleware order follows BACKEND.md §3.
package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/rizwansammo/MateCollab/services/internal/clientconfig"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

// Router returns the /api/v1 router with the platform middleware chain.
func Router(cfg *config.Config, logger *slog.Logger, m *telemetry.Metrics) http.Handler {
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
	})
	return r
}
