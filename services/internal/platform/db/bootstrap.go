package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// BootstrapOptions names the connections used to create roles and the database.
type BootstrapOptions struct {
	SuperuserURL string // e.g. postgres://postgres:…@host/postgres
	AppURL       string // runtime role, target database
	MigratorURL  string // migration role, target database
}

// Bootstrap creates or updates the database roles and the application database. It needs
// superuser rights (BYPASSRLS can only be granted by a superuser), so it is a separate step from
// migrations: the PostgreSQL superuser is used here and nowhere else. It is idempotent and
// re-synchronises role passwords with the configured URLs.
func Bootstrap(ctx context.Context, o BootstrapOptions, logger *slog.Logger) error {
	app, err := pgx.ParseConfig(o.AppURL)
	if err != nil {
		return errors.New("bootstrap: MC_DATABASE_URL could not be parsed")
	}
	mig, err := pgx.ParseConfig(o.MigratorURL)
	if err != nil {
		return errors.New("bootstrap: MC_MIGRATE_DATABASE_URL could not be parsed")
	}
	if app.Database == "" || app.Database != mig.Database {
		return errors.New("bootstrap: the runtime and migration URLs must name the same database")
	}
	if app.User == mig.User || app.User == OwnerRole || mig.User == OwnerRole {
		return errors.New("bootstrap: runtime, migration and owner roles must be distinct")
	}

	su, err := pgx.Connect(ctx, o.SuperuserURL)
	if err != nil {
		return fmt.Errorf("bootstrap: cannot connect as superuser: %w", err)
	}
	defer su.Close(context.WithoutCancel(ctx))

	// format() takes VARIADIC "any", so every parameter is cast to text explicitly.
	steps := []struct {
		desc string
		sql  string
		args []any
	}{
		{"owner role", `SELECT format(CASE WHEN EXISTS (SELECT FROM pg_roles WHERE rolname = $1::text)
			THEN 'ALTER ROLE %I NOLOGIN BYPASSRLS' ELSE 'CREATE ROLE %I NOLOGIN BYPASSRLS' END, $1::text)`, []any{OwnerRole}},
		{"migration role", `SELECT format(CASE WHEN EXISTS (SELECT FROM pg_roles WHERE rolname = $1::text)
			THEN 'ALTER ROLE %I LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L'
			ELSE 'CREATE ROLE %I LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD %L' END, $1::text, $2::text)`, []any{mig.User, mig.Password}},
		{"migration role membership", `SELECT format('GRANT %I TO %I', $1::text, $2::text)`, []any{OwnerRole, mig.User}},
		{"runtime role", `SELECT format(CASE WHEN EXISTS (SELECT FROM pg_roles WHERE rolname = $1::text)
			THEN 'ALTER ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS PASSWORD %L'
			ELSE 'CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS PASSWORD %L' END, $1::text, $2::text)`, []any{app.User, app.Password}},
		{"database", `SELECT format(CASE WHEN EXISTS (SELECT FROM pg_database WHERE datname = $1::text)
			THEN 'ALTER DATABASE %I OWNER TO %I' ELSE 'CREATE DATABASE %I OWNER %I' END, $1::text, $2::text)`, []any{app.Database, OwnerRole}},
		{"database privileges", `SELECT format('REVOKE ALL ON DATABASE %1$I FROM PUBLIC; GRANT CONNECT ON DATABASE %1$I TO %2$I, %3$I',
			$1::text, $2::text, $3::text)`, []any{app.Database, app.User, mig.User}},
	}
	for _, s := range steps {
		var stmt string
		if err := su.QueryRow(ctx, s.sql, s.args...).Scan(&stmt); err != nil {
			return fmt.Errorf("bootstrap %s: %w", s.desc, err)
		}
		if _, err := su.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("bootstrap %s: %w", s.desc, err)
		}
		logger.Info("bootstrap", slog.String("step", s.desc))
	}

	// Schema-level settings inside the application database.
	target := su.Config().Copy()
	target.Database = app.Database
	conn, err := pgx.ConnectConfig(ctx, target)
	if err != nil {
		return fmt.Errorf("bootstrap: cannot connect to %s: %w", app.Database, err)
	}
	defer conn.Close(context.WithoutCancel(ctx))
	var stmt string
	if err := conn.QueryRow(ctx, `SELECT format(
		'ALTER SCHEMA public OWNER TO %1$I; REVOKE CREATE ON SCHEMA public FROM PUBLIC; CREATE EXTENSION IF NOT EXISTS pg_trgm',
		$1::text)`, OwnerRole).Scan(&stmt); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("bootstrap schema: %w", err)
	}
	logger.Info("bootstrap", slog.String("step", "schema and extensions"), slog.String("database", app.Database))
	return nil
}
