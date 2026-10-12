// Command worker runs MateCollab background jobs on River (ADR-004).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"github.com/rizwansammo/MateCollab/services/internal/app"
	"github.com/rizwansammo/MateCollab/services/internal/platform/adminserver"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/health"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := adminserver.Probe(workerEnv("MC_ADMIN_ADDR")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load(workerEnv)
	if err != nil {
		return err
	}
	if err := cfg.Require(config.PartAdmin, config.PartDatabase, config.PartRedis); err != nil {
		return err
	}
	version := app.BuildVersion()
	logger := telemetry.NewLogger(cfg.Log, "worker", version)
	slog.SetDefault(logger)

	shutdownTracing, err := telemetry.InitTracing(ctx, cfg.Telemetry.OTLPEndpoint, "matecollab-worker", version, string(cfg.Env))
	if err != nil {
		return fmt.Errorf("tracing: %w", err)
	}
	defer func() { _ = shutdownTracing(context.WithoutCancel(ctx)) }()

	pool, err := db.NewPool(ctx, cfg.Database.URL, db.PoolOptions{ApplicationName: "mc-worker", MaxConns: 15})
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

	metrics := telemetry.NewMetrics("worker")
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.HeartbeatWorker{Metrics: metrics, Logger: logger})

	client, err := jobs.NewWorkerClient(pool, logger, workers, []*river.PeriodicJob{jobs.HeartbeatPeriodic()})
	if err != nil {
		return fmt.Errorf("river: %w", err)
	}
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("river start: %w", err)
	}
	logger.Info("worker started", slog.Int("queues", len(jobs.Queues)))

	checks := []health.Check{
		{Name: "postgres", Critical: true, Probe: pool.Ping},
		{Name: "valkey", Critical: true, Probe: func(ctx context.Context) error { return redisx.Ping(ctx, rdb) }},
	}
	serveErr := httpx.Serve(ctx, logger, httpx.NewServer(cfg.AdminAddr, adminserver.Handler(metrics, checks)))

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Warn("river stop", slog.Any("error", err))
	}
	logger.Info("worker stopped")
	return serveErr
}

// workerEnv gives the worker its own admin port so it can run beside the API on one host.
func workerEnv(k string) string {
	if v := os.Getenv(k); v != "" || k != "MC_ADMIN_ADDR" {
		return v
	}
	return ":8092"
}
