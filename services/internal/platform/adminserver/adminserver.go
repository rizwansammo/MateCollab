// Package adminserver is the internal listener for liveness, readiness and metrics. It is never
// routed publicly (OPERATIONS.md §6).
package adminserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rizwansammo/MateCollab/services/internal/platform/health"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

// Handler serves /healthz, /readyz and /metrics.
func Handler(m *telemetry.Metrics, checks []health.Check) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID, httpx.Recover)
	r.Method(http.MethodGet, "/healthz", health.Liveness())
	r.Method(http.MethodGet, "/readyz", health.Readiness(checks))
	r.Method(http.MethodGet, "/metrics", m.Handler())
	r.NotFound(httpx.NotFoundHandler)
	return r
}

// Probe performs the container health check (`api healthcheck`, `worker healthcheck`): distroless
// images have no curl, so the binary checks its own readiness endpoint.
func Probe(adminAddr string) error {
	host := adminAddr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ready (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
