//go:build integration

package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

// A job inserted in a committed transaction is worked; one inserted in a rolled-back transaction
// never exists (the guarantee behind ADR-004).
func TestTransactionalEnqueueAndWork(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metrics := telemetry.NewMetrics("test")
	workers := river.NewWorkers()
	river.AddWorker(workers, &jobs.HeartbeatWorker{Metrics: metrics, Logger: testkit.Logger(t)})
	client, err := jobs.NewWorkerClient(d.App, testkit.Logger(t), workers, nil)
	if err != nil {
		t.Fatal(err)
	}
	events, unsubscribe := client.Subscribe(river.EventKindJobCompleted)
	defer unsubscribe()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Stop(context.Background()) }()

	// Rolled back: the job must not exist.
	_ = db.InTx(ctx, d.App, func(tx *db.Tx) error {
		if _, err := client.InsertTx(ctx, tx, jobs.HeartbeatArgs{}, &river.InsertOpts{Queue: "maintenance"}); err != nil {
			t.Fatal(err)
		}
		return context.Canceled
	})
	var n int
	if err := d.Owner.QueryRow(ctx, `SELECT count(*) FROM river_job`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rolled-back insert left %d jobs", n)
	}

	// Committed: the job is worked.
	if err := db.InTx(ctx, d.App, func(tx *db.Tx) error {
		_, err := client.InsertTx(ctx, tx, jobs.HeartbeatArgs{}, &river.InsertOpts{Queue: "maintenance"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-events:
		if ev.Job.Kind != "platform.heartbeat" {
			t.Fatalf("unexpected job %s", ev.Job.Kind)
		}
	case <-ctx.Done():
		t.Fatal("heartbeat job was not worked")
	}
}
