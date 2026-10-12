// Package testkit provides real-dependency fixtures for integration tests (TESTING.md). Tests run
// against a throwaway PostgreSQL cluster with production roles and row-level security active —
// never against a developer's working database, because roles are cluster-wide.
package testkit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
)

// Env names read by integration tests.
const (
	EnvBootstrapURL = "MC_TEST_DATABASE_BOOTSTRAP_URL" // superuser URL of a throwaway cluster
	EnvRolePassword = "MC_TEST_ROLE_PASSWORD"          // password given to mc_app / mc_migrator there
	EnvRedisURL     = "MC_TEST_REDIS_URL"
	EnvMailpitAPI   = "MC_TEST_MAILPIT_API" // e.g. http://localhost:8025
	EnvSMTPHost     = "MC_TEST_SMTP_HOST"
	EnvSMTPPort     = "MC_TEST_SMTP_PORT"
	EnvS3Endpoint   = "MC_TEST_S3_ENDPOINT"
	EnvS3AccessKey  = "MC_TEST_S3_ACCESS_KEY_ID"
	EnvS3SecretKey  = "MC_TEST_S3_SECRET_ACCESS_KEY"
)

// Require returns the named variable or skips the test.
func Require(t testing.TB, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s not set; integration dependency unavailable", name)
	}
	return v
}

// Logger discards output unless the test runs verbosely.
func Logger(t testing.TB) *slog.Logger {
	if testing.Verbose() {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Database is an isolated, bootstrapped and migrated database.
type Database struct {
	Name     string
	App      *pgxpool.Pool // mc_app: RLS applies
	Owner    *pgxpool.Pool // mc_migrator acting as mc_owner: for fixtures and schema
	AppURL   string
	MigURL   string
	Migrator *db.Migrator
}

// NewDatabase creates a fresh database, runs bootstrap and all migrations, and drops it after the test.
func NewDatabase(t testing.TB) *Database {
	t.Helper()
	ctx := context.Background()
	superURL := Require(t, EnvBootstrapURL)
	password := Require(t, EnvRolePassword)

	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "mc_test_" + hex.EncodeToString(b[:])
	appURL, migURL := withCredentials(t, superURL, "mc_app", password, name), withCredentials(t, superURL, "mc_migrator", password, name)

	logger := Logger(t)
	if err := db.Bootstrap(ctx, db.BootstrapOptions{SuperuserURL: superURL, AppURL: appURL, MigratorURL: migURL}, logger); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { dropDatabase(t, superURL, name) })

	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	m, err := db.NewMigrator(ctx, migURL, cfg.Database.MigrationsDir, logger)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	t.Cleanup(m.Close)
	if err := m.Up(ctx); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	app, err := db.NewPool(ctx, appURL, db.PoolOptions{ApplicationName: "mc-test", MaxConns: 4})
	if err != nil {
		t.Fatalf("app pool: %v", err)
	}
	t.Cleanup(app.Close)
	owner, err := db.NewPool(ctx, migURL, db.PoolOptions{ApplicationName: "mc-test-owner", MaxConns: 2, SetRole: db.OwnerRole})
	if err != nil {
		t.Fatalf("owner pool: %v", err)
	}
	t.Cleanup(owner.Close)
	return &Database{Name: name, App: app, Owner: owner, AppURL: appURL, MigURL: migURL, Migrator: m}
}

func withCredentials(t testing.TB, base, user, password, database string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvBootstrapURL, err)
	}
	u.User = url.UserPassword(user, password)
	u.Path = "/" + database
	return u.String()
}

func dropDatabase(t testing.TB, superURL, name string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, superURL)
	if err != nil {
		t.Logf("cleanup: %v", err)
		return
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", pgx.Identifier{name}.Sanitize())); err != nil {
		t.Logf("cleanup drop %s: %v", name, err)
	}
}
