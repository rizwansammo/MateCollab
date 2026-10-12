// Package jobs is the background-job foundation on River and PostgreSQL (ADR-004). Jobs are
// inserted in the same transaction as the change that causes them, so they are durable exactly
// when the change commits. Redis is never used for durable work.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// Queue names and initial concurrency (BACKEND.md §4).
var Queues = map[string]river.QueueConfig{
	river.QueueDefault: {MaxWorkers: 20},
	"notifications":    {MaxWorkers: 20},
	"email":            {MaxWorkers: 10},
	"files":            {MaxWorkers: 4},
	"support":          {MaxWorkers: 5},
	"maintenance":      {MaxWorkers: 2},
}

// Client is the River client type used across MateCollab.
type Client = river.Client[pgx.Tx]

// Enqueuer inserts jobs inside an existing transaction. Services depend on this interface, not
// on River directly, so the engine stays replaceable.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// NewInsertOnlyClient returns a client that can enqueue but does not work jobs (API processes).
func NewInsertOnlyClient(pool *pgxpool.Pool, logger *slog.Logger) (*Client, error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{Logger: logger})
}

// NewWorkerClient returns a client that works every queue with the registered workers and the
// platform periodic jobs.
func NewWorkerClient(pool *pgxpool.Pool, logger *slog.Logger, workers *river.Workers, periodic []*river.PeriodicJob) (*Client, error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:       logger,
		Queues:       Queues,
		Workers:      workers,
		PeriodicJobs: periodic,
		JobTimeout:   5 * time.Minute,
	})
}

// HeartbeatArgs is the platform heartbeat: it proves the worker is consuming jobs end to end.
type HeartbeatArgs struct{}

// Kind implements river.JobArgs.
func (HeartbeatArgs) Kind() string { return "platform.heartbeat" }

// InsertOpts routes heartbeats to the maintenance queue and keeps one pending at a time.
func (HeartbeatArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "maintenance", UniqueOpts: river.UniqueOpts{ByPeriod: time.Minute}}
}

// HeartbeatWorker records the time of the last completed heartbeat.
type HeartbeatWorker struct {
	river.WorkerDefaults[HeartbeatArgs]
	Metrics *telemetry.Metrics
	Logger  *slog.Logger
}

// Work implements river.Worker.
func (w *HeartbeatWorker) Work(ctx context.Context, job *river.Job[HeartbeatArgs]) error {
	if w.Metrics != nil {
		w.Metrics.WorkerHeartbeat.SetToCurrentTime()
	}
	w.Logger.DebugContext(ctx, "worker heartbeat", slog.Int64("job_id", job.ID))
	return nil
}

// HeartbeatPeriodic schedules the heartbeat every minute, starting immediately.
func HeartbeatPeriodic() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return HeartbeatArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

// TenantArgs is implemented by job arguments that act on one organisation's data.
type TenantArgs interface {
	river.JobArgs
	TenantOrgID() uuid.UUID
}

// InTenant runs a job body inside a tenant transaction for the job's organisation, so row-level
// security applies to background work exactly as to API requests.
func InTenant[T TenantArgs](pool *pgxpool.Pool, fn func(context.Context, *db.Tx, *river.Job[T]) error) func(context.Context, *river.Job[T]) error {
	return func(ctx context.Context, job *river.Job[T]) error {
		scope := tenant.Scope{OrgID: tenant.OrgID(job.Args.TenantOrgID())}
		return db.InTenantTx(ctx, pool, scope, func(tx *db.Tx) error { return fn(ctx, tx, job) })
	}
}
