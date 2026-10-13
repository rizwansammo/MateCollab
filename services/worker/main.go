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
	"github.com/rizwansammo/MateCollab/services/internal/audit"
	"github.com/rizwansammo/MateCollab/services/internal/emails"
	"github.com/rizwansammo/MateCollab/services/internal/platform/adminserver"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/crypto"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/health"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/idempotency"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mailer"
	"github.com/rizwansammo/MateCollab/services/internal/platform/objectstore"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
	"github.com/rizwansammo/MateCollab/services/internal/users"
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
	// Timestamps leave the process in UTC (API-DESIGN.md §4) whatever the host's zone; the
	// database driver returns times in time.Local.
	time.Local = time.UTC
	cfg, err := config.Load(workerEnv)
	if err != nil {
		return err
	}
	if err := cfg.Require(config.PartAdmin, config.PartDatabase, config.PartRedis, config.PartSMTP, config.PartCrypto, config.PartStorage); err != nil {
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

	keyring, err := crypto.ParseKeyring(cfg.Crypto.Keyring, cfg.Crypto.ActiveKey)
	if err != nil {
		return fmt.Errorf("keyring: %w", err)
	}
	smtp, err := mailer.New(cfg.SMTP)
	if err != nil {
		return err
	}

	metrics := telemetry.NewMetrics("worker")
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.HeartbeatWorker{Metrics: metrics, Logger: logger})
	river.AddWorker(workers, &emails.Worker{Sender: smtp, Keyring: keyring, Logger: logger})
	river.AddWorker(workers, &users.PurgeWorker{Pool: pool, Logger: logger})
	river.AddWorker(workers, &idempotency.CleanupWorker{Pool: pool})
	storage, err := objectstore.NewS3(cfg.Storage)
	if err != nil {
		return err
	}
	exportWorker := &audit.ExportWorker{Pool: pool, Storage: storage, Logger: logger}
	river.AddWorker(workers, exportWorker)
	river.AddWorker(workers, &audit.ExpireWorker{Pool: pool, Storage: storage})

	client, err := jobs.NewWorkerClient(pool, logger, workers, []*river.PeriodicJob{jobs.HeartbeatPeriodic(), users.PurgePeriodic(), idempotency.CleanupPeriodic()})
	if err != nil {
		return fmt.Errorf("river: %w", err)
	}
	exportWorker.Jobs = client // schedules the expiry of finished exports
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
