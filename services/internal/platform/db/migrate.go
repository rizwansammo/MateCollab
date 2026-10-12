package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

var migrationFile = regexp.MustCompile(`^(\d+)_[a-z0-9_]+\.sql$`)

// Migrator applies goose SQL migrations from db/migrations and River's own schema, as the
// migration role acting as the owner role. It is never run by the API or worker at start-up.
type Migrator struct {
	pool   *pgxpool.Pool
	db     *sql.DB
	goose  *goose.Provider
	logger *slog.Logger
}

// NewMigrator connects with the migration URL and prepares the goose provider.
func NewMigrator(ctx context.Context, migrateURL, dir string, logger *slog.Logger) (*Migrator, error) {
	if dir == "" {
		return nil, errors.New("migrate: migrations directory not found (set MC_MIGRATIONS_DIR)")
	}
	pool, err := NewPool(ctx, migrateURL, PoolOptions{ApplicationName: "mc-mcctl", MaxConns: 4, SetRole: OwnerRole})
	if err != nil {
		return nil, err
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS(dir))
	if err != nil {
		_ = sqlDB.Close()
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Migrator{pool: pool, db: sqlDB, goose: provider, logger: logger}, nil
}

// Close releases connections.
func (m *Migrator) Close() {
	_ = m.db.Close()
	m.pool.Close()
}

// Up applies all pending application migrations, then River's job-queue schema.
func (m *Migrator) Up(ctx context.Context) error {
	results, err := m.goose.Up(ctx)
	for _, r := range results {
		m.logger.Info("migration applied", slog.Int64("version", r.Source.Version), slog.String("file", filepath.Base(r.Source.Path)), slog.Duration("took", r.Duration))
	}
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	if len(results) == 0 {
		m.logger.Info("application schema is up to date")
	}
	rm, err := rivermigrate.New(riverpgxv5.New(m.pool), nil)
	if err != nil {
		return fmt.Errorf("migrate river: %w", err)
	}
	res, err := rm.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("migrate river: %w", err)
	}
	for _, v := range res.Versions {
		m.logger.Info("river migration applied", slog.Int("version", v.Version))
	}
	return nil
}

// Down rolls back the most recent application migration (River's schema is left in place).
func (m *Migrator) Down(ctx context.Context) error {
	r, err := m.goose.Down(ctx)
	if r != nil {
		m.logger.Info("migration rolled back", slog.Int64("version", r.Source.Version), slog.String("file", filepath.Base(r.Source.Path)))
	}
	if errors.Is(err, goose.ErrNoNextVersion) {
		m.logger.Info("nothing to roll back")
		return nil
	}
	return err
}

// Status lists every migration and whether it is applied.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	return m.goose.Status(ctx)
}

// ExpectedVersion returns the highest migration version present in dir.
func ExpectedVersion(dir string) (int64, error) {
	entries, err := fs.ReadDir(os.DirFS(dir), ".")
	if err != nil {
		return 0, fmt.Errorf("migrations directory: %w", err)
	}
	var max int64
	for _, e := range entries {
		if mm := migrationFile.FindStringSubmatch(e.Name()); mm != nil {
			v, _ := strconv.ParseInt(mm[1], 10, 64)
			if v > max {
				max = v
			}
		}
	}
	return max, nil
}

// CurrentVersion returns the highest applied application migration (0 when none).
func CurrentVersion(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var v int64
	err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied AND version_id > 0`).Scan(&v)
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "42P01" { // undefined_table
			return 0, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

// ErrSchemaBehind means the database needs `mcctl migrate up` before this build can run.
var ErrSchemaBehind = errors.New("database schema is behind this build; run `pnpm db:migrate` (mcctl migrate up)")

// CheckSchema refuses to start a process against a database that is behind its migrations.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	want, err := ExpectedVersion(dir)
	if err != nil {
		return err
	}
	got, err := CurrentVersion(ctx, pool)
	if err != nil {
		return fmt.Errorf("schema check: %w", err)
	}
	if got < want {
		return fmt.Errorf("%w (database at %d, build expects %d)", ErrSchemaBehind, got, want)
	}
	return nil
}
