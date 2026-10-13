// Command api runs the MateCollab HTTP API (and, later, the WebSocket gateway).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // time zone validation must not depend on the container's zoneinfo

	"github.com/rizwansammo/MateCollab/services/internal/apiserver"
	"github.com/rizwansammo/MateCollab/services/internal/app"
	"github.com/rizwansammo/MateCollab/services/internal/platform/adminserver"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/health"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/objectstore"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := adminserver.Probe(envOr("MC_ADMIN_ADDR", ":8091")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Timestamps leave the process in UTC (API-DESIGN.md §4) whatever the host's zone; the
	// database driver returns times in time.Local.
	time.Local = time.UTC
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if err := cfg.Require(config.PartHTTP, config.PartAdmin, config.PartDatabase, config.PartRedis, config.PartCrypto, config.PartStorage); err != nil {
		return err
	}
	version := app.BuildVersion()
	logger := telemetry.NewLogger(cfg.Log, "api", version)
	slog.SetDefault(logger)

	shutdownTracing, err := telemetry.InitTracing(ctx, cfg.Telemetry.OTLPEndpoint, "matecollab-api", version, string(cfg.Env))
	if err != nil {
		return fmt.Errorf("tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.WithoutCancel(ctx)) }()

	pool, err := db.NewPool(ctx, cfg.Database.URL, db.PoolOptions{ApplicationName: "mc-api", MaxConns: cfg.Database.MaxConns, StatementTimeout: statementTimeout})
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.CheckSchema(ctx, pool, cfg.Database.MigrationsDir); err != nil {
		return err
	}

	rdb, err := redisx.New(cfg.Redis.URL)
	if err != nil {
		return err
	}
	defer rdb.Close()

	// The API only enqueues jobs (emails and, later, fan-out work); the worker runs them.
	queue, err := jobs.NewInsertOnlyClient(pool, logger)
	if err != nil {
		return fmt.Errorf("river: %w", err)
	}
	storage, err := objectstore.NewS3(cfg.Storage)
	if err != nil {
		return err
	}
	svc, err := apiserver.NewServices(apiserver.Deps{Config: cfg, Pool: pool, Redis: rdb, Jobs: queue, Storage: storage, Logger: logger})
	if err != nil {
		return err
	}

	metrics := telemetry.NewMetrics("api")
	checks := []health.Check{
		{Name: "postgres", Critical: true, Probe: pool.Ping},
		{Name: "valkey", Critical: true, Probe: func(ctx context.Context) error { return redisx.Ping(ctx, rdb) }},
	}

	logger.Info("api starting", slog.String("env", string(cfg.Env)), slog.String("public_url", cfg.PublicURL.String()))
	return httpx.Serve(ctx, logger,
		httpx.NewServer(cfg.HTTPAddr, apiserver.Router(cfg, logger, metrics, svc)),
		httpx.NewServer(cfg.AdminAddr, adminserver.Handler(metrics, checks)),
	)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
