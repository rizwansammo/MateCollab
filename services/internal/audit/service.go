package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/rizwansammo/MateCollab/services/internal/audit/store"
	"github.com/rizwansammo/MateCollab/services/internal/authz"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/cursor"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/httpx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/objectstore"
	"github.com/rizwansammo/MateCollab/services/internal/platform/ratelimit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// ExportTTL is how long a finished export can be downloaded (FILES.md: exports expire after 7 days).
const ExportTTL = 7 * 24 * time.Hour

var limitExportsPerOrg = ratelimit.PerHour(10)

// Deps are the collaborators of the audit service.
type Deps struct {
	Pool      *pgxpool.Pool
	Jobs      jobs.Enqueuer
	Storage   objectstore.Store // nil when object storage is not configured: exports are unavailable
	Limiter   *ratelimit.Limiter
	CursorKey []byte
	Logger    *slog.Logger
	Now       func() time.Time
}

// Service reads the organisation audit log and produces CSV exports.
type Service struct{ Deps }

// New returns an audit service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{Deps: d}
}

// Filter narrows the audit log. It is stored with an export so the job reproduces the query.
type Filter struct {
	ActorUserID *uuid.UUID `json:"actor_user_id,omitempty"`
	Action      string     `json:"action,omitempty"`
	TargetType  string     `json:"target_type,omitempty"`
	TargetID    string     `json:"target_id,omitempty"`
	From        *time.Time `json:"from,omitempty"`
	To          *time.Time `json:"to,omitempty"`
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (f Filter) params(org tenant.OrgID, limit int) store.ListAuditLogsParams {
	return store.ListAuditLogsParams{OrgID: uuid.UUID(org), ActorUserID: f.ActorUserID, Action: optional(f.Action),
		TargetType: optional(f.TargetType), TargetID: optional(f.TargetID), CreatedFrom: f.From, CreatedTo: f.To,
		RowLimit: int32(limit)}
}

// Item is one audit log entry as shown to administrators.
type Item = store.ListAuditLogsRow

type pageCursor struct {
	CreatedAt time.Time `json:"c"`
	ID        uuid.UUID `json:"i"`
}

func (f Filter) scope(org tenant.OrgID) string {
	raw, _ := json.Marshal(f)
	return "audit:" + org.String() + ":" + string(raw)
}

// List returns one page of the audit log, newest first, and the next cursor.
func (s *Service) List(ctx context.Context, a tenant.Actor, f Filter, cur string, limit int) ([]Item, string, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.AuditView); err != nil {
		return nil, "", err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	params := f.params(a.OrgID, limit+1)
	if cur != "" {
		var c pageCursor
		if err := cursor.Decode(s.CursorKey, f.scope(a.OrgID), cur, &c); err != nil {
			return nil, "", err
		}
		params.BeforeCreated, params.BeforeID = &c.CreatedAt, &c.ID
	}
	var rows []Item
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		rows, err = store.New(tx).ListAuditLogs(ctx, params)
		return err
	})
	if err != nil || len(rows) <= limit {
		return rows, "", err
	}
	last := rows[limit-1]
	next, err := cursor.Encode(s.CursorKey, f.scope(a.OrgID), pageCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	return rows[:limit], next, err
}

// Export is an export request and its progress.
type Export struct {
	ID          uuid.UUID
	Status      string
	Filter      Filter
	RowCount    *int32
	Error       *string
	CreatedAt   time.Time
	CompletedAt *time.Time
	ExpiresAt   time.Time
}

var errExportsUnavailable = &apperr.Error{Kind: apperr.KindUnavailable, Code: "exports_unavailable",
	Detail: "Exports need object storage, which is not configured on this server."}

// RequestExport queues a CSV export of the filtered audit log (BACKEND.md: `audit.export`).
func (s *Service) RequestExport(ctx context.Context, a tenant.Actor, f Filter, meta httpx.ClientMeta) (Export, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.AuditExport); err != nil {
		return Export{}, err
	}
	if s.Storage == nil {
		return Export{}, errExportsUnavailable
	}
	if err := s.Limiter.Check(ctx, "audit.export", a.OrgID.String(), limitExportsPerOrg); err != nil {
		return Export{}, err
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return Export{}, err
	}
	var out Export
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		row, err := store.New(tx).CreateAuditExport(ctx, store.CreateAuditExportParams{
			OrgID: uuid.UUID(a.OrgID), RequestedBy: uuid.UUID(a.UserID), Filters: raw, ExpiresAt: s.Now().Add(ExportTTL),
		})
		if err != nil {
			return err
		}
		out = Export{ID: row.ID, Status: row.Status, Filter: f, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt}
		if _, err := s.Jobs.InsertTx(ctx, tx, ExportArgs{OrgID: uuid.UUID(a.OrgID), ExportID: row.ID}, nil); err != nil {
			return err
		}
		return RecordActor(ctx, tx, a, meta, Entry{Action: "audit.export_requested", TargetType: "audit_export", TargetID: row.ID.String()})
	})
	return out, err
}

func toExport(id uuid.UUID, status string, filters json.RawMessage, rows *int32, errText *string, created time.Time, completed *time.Time, expires time.Time) Export {
	e := Export{ID: id, Status: status, RowCount: rows, Error: errText, CreatedAt: created, CompletedAt: completed, ExpiresAt: expires}
	_ = json.Unmarshal(filters, &e.Filter)
	return e
}

// ListExports returns the organisation's 20 most recent exports.
func (s *Service) ListExports(ctx context.Context, a tenant.Actor) ([]Export, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.AuditExport); err != nil {
		return nil, err
	}
	var out []Export
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		rows, err := store.New(tx).ListAuditExports(ctx, uuid.UUID(a.OrgID))
		for _, r := range rows {
			out = append(out, toExport(r.ID, r.Status, r.Filters, r.RowCount, r.Error, r.CreatedAt, r.CompletedAt, r.ExpiresAt))
		}
		return err
	})
	return out, err
}

func (s *Service) getExport(ctx context.Context, a tenant.Actor, id uuid.UUID) (store.GetAuditExportRow, error) {
	var row store.GetAuditExportRow
	err := db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		var err error
		row, err = store.New(tx).GetAuditExport(ctx, store.GetAuditExportParams{OrgID: uuid.UUID(a.OrgID), ExportID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound()
		}
		return err
	})
	return row, err
}

// GetExport returns one export.
func (s *Service) GetExport(ctx context.Context, a tenant.Actor, id uuid.UUID) (Export, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.AuditExport); err != nil {
		return Export{}, err
	}
	r, err := s.getExport(ctx, a, id)
	if err != nil {
		return Export{}, err
	}
	return toExport(r.ID, r.Status, r.Filters, r.RowCount, r.Error, r.CreatedAt, r.CompletedAt, r.ExpiresAt), nil
}

// DownloadURL returns a 60-second presigned download URL for a finished export (FILES.md §6).
func (s *Service) DownloadURL(ctx context.Context, a tenant.Actor, id uuid.UUID, slug string, meta httpx.ClientMeta) (string, error) {
	if err := authz.Check(a, authz.SettingsFrom(ctx), authz.AuditExport); err != nil {
		return "", err
	}
	r, err := s.getExport(ctx, a, id)
	if err != nil {
		return "", err
	}
	// Resolve the tenant-scoped resource before checking optional infrastructure so a caller
	// can never distinguish another organisation's export from a missing one.
	if s.Storage == nil {
		return "", errExportsUnavailable
	}
	if r.Status != "completed" || r.ObjectKey == nil || !s.Now().Before(r.ExpiresAt) {
		return "", apperr.Conflict("export_not_ready", "This export is not available for download.")
	}
	url, err := s.Storage.PresignGet(ctx, *r.ObjectKey, 60*time.Second, objectstore.DownloadOptions{
		Filename: fmt.Sprintf("audit-log-%s-%s.csv", slug, r.CreatedAt.UTC().Format("2006-01-02")), ContentType: "text/csv",
	})
	if err != nil {
		return "", err
	}
	err = db.InTenantTx(ctx, s.Pool, a.Scope(), func(tx *db.Tx) error {
		return RecordActor(ctx, tx, a, meta, Entry{Action: "audit.export_downloaded", TargetType: "audit_export", TargetID: id.String()})
	})
	return url, err
}

// ---- jobs ---------------------------------------------------------------------------------------

// ExportArgs is the `audit.export` job.
type ExportArgs struct {
	OrgID    uuid.UUID `json:"org_id"`
	ExportID uuid.UUID `json:"export_id"`
}

// Kind is the River job kind.
func (ExportArgs) Kind() string { return "audit.export" }

// InsertOpts routes exports to the maintenance queue.
func (ExportArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: "maintenance", MaxAttempts: 3}
}

// TenantOrgID scopes the job to its organisation.
func (a ExportArgs) TenantOrgID() uuid.UUID { return a.OrgID }

// ExpireArgs deletes an export's object when its download window ends.
type ExpireArgs struct {
	OrgID    uuid.UUID `json:"org_id"`
	ExportID uuid.UUID `json:"export_id"`
}

// Kind is the River job kind.
func (ExpireArgs) Kind() string { return "audit.export_expire" }

// InsertOpts routes expiry to the maintenance queue.
func (ExpireArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: "maintenance"} }

// TenantOrgID scopes the job to its organisation.
func (a ExpireArgs) TenantOrgID() uuid.UUID { return a.OrgID }

// ExportWorker writes the filtered audit log to CSV in object storage.
type ExportWorker struct {
	river.WorkerDefaults[ExportArgs]
	Pool    *pgxpool.Pool
	Storage objectstore.Store
	Jobs    jobs.Enqueuer
	Logger  *slog.Logger
}

// Work streams the rows page by page into a temporary file, uploads it and schedules expiry.
func (w *ExportWorker) Work(ctx context.Context, job *river.Job[ExportArgs]) error {
	org := tenant.OrgID(job.Args.OrgID)
	scope := tenant.Scope{OrgID: org}
	var f Filter
	var expires time.Time
	err := db.InTenantTx(ctx, w.Pool, scope, func(tx *db.Tx) error {
		row, err := store.New(tx).ClaimAuditExport(ctx, store.ClaimAuditExportParams{OrgID: job.Args.OrgID, ExportID: job.Args.ExportID})
		if err != nil {
			return err
		}
		expires = row.ExpiresAt
		return json.Unmarshal(row.Filters, &f)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // already finished or expired
	}
	if err != nil {
		return err
	}

	file, err := os.CreateTemp("", "mc-audit-export-*.csv")
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	count, err := w.writeCSV(ctx, org, f, file)
	if err == nil {
		var size int64
		if size, err = file.Seek(0, io.SeekCurrent); err == nil {
			if _, err = file.Seek(0, io.SeekStart); err == nil {
				key := fmt.Sprintf("orgs/%s/exports/%s.csv", org, job.Args.ExportID)
				if err = w.Storage.Put(ctx, key, file, size, "text/csv"); err == nil {
					err = db.InTenantTx(ctx, w.Pool, scope, func(tx *db.Tx) error {
						n := int32(count)
						if err := store.New(tx).CompleteAuditExport(ctx, store.CompleteAuditExportParams{
							OrgID: job.Args.OrgID, ExportID: job.Args.ExportID, ObjectKey: &key, RowCount: &n,
						}); err != nil {
							return err
						}
						_, err := w.Jobs.InsertTx(ctx, tx, ExpireArgs(job.Args), &river.InsertOpts{ScheduledAt: expires})
						return err
					})
				}
			}
		}
	}
	if err != nil && job.Attempt >= job.MaxAttempts {
		msg := "The export could not be created."
		_ = db.InTenantTx(context.WithoutCancel(ctx), w.Pool, scope, func(tx *db.Tx) error {
			return store.New(tx).FailAuditExport(ctx, store.FailAuditExportParams{OrgID: job.Args.OrgID, ExportID: job.Args.ExportID, Error: &msg})
		})
	}
	if err == nil {
		w.Logger.InfoContext(ctx, "audit export completed", slog.String("org_id", org.String()), slog.Int("rows", count))
	}
	return err
}

var csvHeader = []string{"time", "actor", "actor_user_id", "actor_type", "action", "outcome", "target_type", "target_id", "ip", "request_id", "details"}

// writeCSV pages through the log (oldest pages last, as listed) in tenant transactions.
func (w *ExportWorker) writeCSV(ctx context.Context, org tenant.OrgID, f Filter, out io.Writer) (int, error) {
	cw := csv.NewWriter(out)
	if err := cw.Write(csvHeader); err != nil {
		return 0, err
	}
	params := f.params(org, 1000)
	count := 0
	for {
		var rows []Item
		err := db.InTenantTx(ctx, w.Pool, tenant.Scope{OrgID: org}, func(tx *db.Tx) error {
			var err error
			rows, err = store.New(tx).ListAuditLogs(ctx, params)
			return err
		})
		if err != nil {
			return count, err
		}
		for _, r := range rows {
			if err := cw.Write(csvRecord(r)); err != nil {
				return count, err
			}
			count++
		}
		if len(rows) < int(params.RowLimit) {
			break
		}
		last := rows[len(rows)-1]
		params.BeforeCreated, params.BeforeID = &last.CreatedAt, &last.ID
	}
	cw.Flush()
	return count, cw.Error()
}

func csvRecord(r Item) []string {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	actorID, ip := "", ""
	if r.ActorUserID != nil {
		actorID = r.ActorUserID.String()
	}
	if r.Ip != nil {
		ip = r.Ip.String()
	}
	fields := []string{r.CreatedAt.UTC().Format(time.RFC3339), str(r.ActorName), actorID, r.ActorType, r.Action, r.Outcome,
		str(r.TargetType), str(r.TargetID), ip, str(r.RequestID), string(r.Metadata)}
	for i, v := range fields {
		fields[i] = csvSafe(v)
	}
	return fields
}

// csvSafe defuses spreadsheet formula injection: cells that a spreadsheet would evaluate are
// prefixed with an apostrophe.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// ExpireWorker removes an export's file at the end of its download window.
type ExpireWorker struct {
	river.WorkerDefaults[ExpireArgs]
	Pool    *pgxpool.Pool
	Storage objectstore.Store
}

// Work deletes the object and marks the export expired.
func (w *ExpireWorker) Work(ctx context.Context, job *river.Job[ExpireArgs]) error {
	return jobs.InTenant(w.Pool, func(ctx context.Context, tx *db.Tx, job *river.Job[ExpireArgs]) error {
		q := store.New(tx)
		row, err := q.GetAuditExport(ctx, store.GetAuditExportParams{OrgID: job.Args.OrgID, ExportID: job.Args.ExportID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.ObjectKey != nil {
			if err := w.Storage.Delete(ctx, *row.ObjectKey); err != nil && !errors.Is(err, objectstore.ErrNotFound) {
				return err
			}
		}
		return q.ExpireAuditExport(ctx, store.ExpireAuditExportParams{OrgID: job.Args.OrgID, ExportID: job.Args.ExportID})
	})(ctx, job)
}
