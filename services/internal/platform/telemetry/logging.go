// Package telemetry sets up structured logging, OpenTelemetry tracing and Prometheus metrics.
package telemetry

import (
	"io"
	"log/slog"
	"os"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
)

// NewLogger returns the process logger. Message bodies, tokens and secrets must never be logged;
// callers log identifiers and outcomes only (SECURITY-BASELINE.md §16).
func NewLogger(cfg config.LogConfig, service, version string) *slog.Logger {
	return newLogger(os.Stdout, cfg, service, version)
}

func newLogger(w io.Writer, cfg config.LogConfig, service, version string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.Level}
	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h).With("service", service, "version", version)
}
