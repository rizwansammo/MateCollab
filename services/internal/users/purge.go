package users

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// PurgeArgs is the periodic job that anonymises accounts whose deletion grace period has ended
// (DATABASE.md §7). Organisation content stays, attributed to "Deleted user".
type PurgeArgs struct{}

// Kind is the River job kind.
func (PurgeArgs) Kind() string { return "users.purge_deleted" }

// InsertOpts runs the purge on the maintenance queue, one at a time.
func (PurgeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "maintenance", UniqueOpts: river.UniqueOpts{ByPeriod: 10 * time.Minute}}
}

// PurgePeriodic schedules the purge hourly (and once at worker start).
func PurgePeriodic() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return PurgeArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

// PurgeWorker anonymises due accounts in batches. The database functions refuse accounts that
// still own an organisation, so an Owner never disappears.
type PurgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Now    func() time.Time
}

// Work anonymises up to 100 due accounts per run.
func (w *PurgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	now := time.Now()
	if w.Now != nil {
		now = w.Now()
	}
	rows, err := w.Pool.Query(ctx, "SELECT mc_users_due_for_anonymisation($1, 100)", now)
	if err != nil {
		return err
	}
	var due []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		due = append(due, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	anonymised, skipped := 0, 0
	for _, id := range due {
		var ok bool
		if err := w.Pool.QueryRow(ctx, "SELECT mc_anonymise_user($1, $2)", id, now).Scan(&ok); err != nil {
			return err
		}
		if ok {
			anonymised++
		} else {
			skipped++ // still an Owner somewhere
			w.Logger.WarnContext(ctx, "account deletion postponed: the account still owns an organisation", slog.String("user_id", id.String()))
		}
	}
	if len(due) > 0 {
		w.Logger.InfoContext(ctx, "account deletion purge", slog.Int("anonymised", anonymised), slog.Int("postponed", skipped))
	}
	return nil
}
