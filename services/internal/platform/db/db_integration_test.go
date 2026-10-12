//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
	"github.com/rizwansammo/MateCollab/services/internal/testkit"
)

func TestBootstrapIsIdempotentAndSchemaIsCurrent(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()

	// A second bootstrap (e.g. after a password rotation) must succeed without changes.
	if err := db.Bootstrap(ctx, db.BootstrapOptions{SuperuserURL: testkit.Require(t, testkit.EnvBootstrapURL), AppURL: d.AppURL, MigratorURL: d.MigURL}, testkit.Logger(t)); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if err := d.Migrator.Up(ctx); err != nil {
		t.Fatalf("second migrate up should be a no-op: %v", err)
	}
	if err := db.CheckSchema(ctx, d.App, migrationsDir(t)); err != nil {
		t.Fatalf("schema check: %v", err)
	}
}

func TestMigrationsRollBackAndReapply(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()
	if err := d.Migrator.Down(ctx); err != nil {
		t.Fatalf("down: %v", err)
	}
	if err := db.CheckSchema(ctx, d.Owner, migrationsDir(t)); !errors.Is(err, db.ErrSchemaBehind) {
		t.Fatalf("schema check after rollback should report behind, got %v", err)
	}
	if err := d.Migrator.Up(ctx); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
}

// Every migration must be reversible: roll the whole application schema back, then forward.
func TestEveryMigrationRollsBack(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()
	statuses, err := d.Migrator.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for range statuses {
		if err := d.Migrator.Down(ctx); err != nil {
			t.Fatalf("down: %v", err)
		}
	}
	var tables int
	if err := d.Owner.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename NOT LIKE 'river%' AND tablename <> 'goose_db_version'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("%d application tables left after rolling everything back", tables)
	}
	if err := d.Migrator.Up(ctx); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
}

// MULTITENANCY.md §2.5: every table with an org_id column has RLS enabled and forced, and at
// least one policy. New tenant tables cannot ship without it.
func TestEveryTenantTableHasRowLevelSecurity(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()
	rows, err := d.Owner.Query(ctx, `
		SELECT c.relname, c.relrowsecurity, c.relforcerowsecurity,
		       (SELECT count(*) FROM pg_policies p WHERE p.schemaname = 'public' AND p.tablename = c.relname)
		FROM information_schema.columns col
		JOIN pg_class c ON c.relname = col.table_name AND c.relnamespace = 'public'::regnamespace AND c.relkind = 'r'
		WHERE col.table_schema = 'public' AND col.column_name = 'org_id'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var name string
		var enabled, forced bool
		var policies int
		if err := rows.Scan(&name, &enabled, &forced, &policies); err != nil {
			t.Fatal(err)
		}
		n++
		if !enabled || !forced || policies == 0 {
			t.Errorf("%s: rls=%t forced=%t policies=%d", name, enabled, forced, policies)
		}
	}
	if n == 0 {
		t.Fatal("no tenant tables found")
	}
}

func TestRuntimeRoleCannotChangeSchemaOrBypassRLS(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()
	if _, err := d.App.Exec(ctx, `CREATE TABLE should_fail (id int)`); !isPGCode(err, "42501") {
		t.Fatalf("mc_app must not create tables, got %v", err)
	}
	var bypass, super bool
	if err := d.App.QueryRow(ctx, `SELECT rolbypassrls, rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&bypass, &super); err != nil {
		t.Fatal(err)
	}
	if bypass || super {
		t.Fatalf("mc_app must not bypass RLS or be superuser (bypass=%t super=%t)", bypass, super)
	}
}

func TestTenantTransactionsAreIsolatedByRLS(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()

	// A probe table built exactly like a tenant table (MULTITENANCY.md §2.5).
	_, err := d.Owner.Exec(ctx, `
		CREATE TABLE rls_probe (id uuid PRIMARY KEY DEFAULT uuidv7(), org_id uuid NOT NULL, body text NOT NULL);
		ALTER TABLE rls_probe ENABLE ROW LEVEL SECURITY;
		ALTER TABLE rls_probe FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation ON rls_probe
		  USING (org_id = mc_current_org_id()) WITH CHECK (org_id = mc_current_org_id());`)
	if err != nil {
		t.Fatalf("probe table: %v", err)
	}
	orgA, orgB := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	if _, err := d.Owner.Exec(ctx, `INSERT INTO rls_probe (org_id, body) VALUES ($1, 'a1'), ($1, 'a2'), ($2, 'b1')`, orgA, orgB); err != nil {
		t.Fatalf("fixtures: %v", err)
	}

	count := func(tx *db.Tx) int {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM rls_probe`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// Organisation A sees only its own rows, even without a WHERE clause.
	err = db.InTenantTx(ctx, d.App, tenant.Scope{OrgID: tenant.OrgID(orgA)}, func(tx *db.Tx) error {
		if n := count(tx); n != 2 {
			t.Errorf("org A sees %d rows, want 2", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Without tenant context RLS fails closed.
	if err := db.InTx(ctx, d.App, func(tx *db.Tx) error {
		if n := count(tx); n != 0 {
			t.Errorf("no tenant context sees %d rows, want 0", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// Writing another organisation's row inside org A's transaction is rejected by WITH CHECK.
	err = db.InTenantTx(ctx, d.App, tenant.Scope{OrgID: tenant.OrgID(orgA)}, func(tx *db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO rls_probe (org_id, body) VALUES ($1, 'smuggled')`, orgB)
		return err
	})
	if !isPGCode(err, "42501") {
		t.Fatalf("cross-tenant insert must be rejected by RLS, got %v", err)
	}

	// Tenant settings are transaction-local: the next transaction on the pool sees nothing.
	if err := db.InTx(ctx, d.App, func(tx *db.Tx) error {
		var org *string
		if err := tx.QueryRow(ctx, `SELECT nullif(current_setting('mc.org_id', true), '')`).Scan(&org); err != nil {
			return err
		}
		if org != nil {
			t.Errorf("tenant setting leaked across transactions: %s", *org)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A missing organisation is refused before touching the database.
	if err := db.InTenantTx(ctx, d.App, tenant.Scope{}, func(*db.Tx) error { return nil }); !errors.Is(err, tenant.ErrNoTenant) {
		t.Fatalf("empty scope must be refused, got %v", err)
	}
}

func TestAfterCommitRunsOnlyAfterCommit(t *testing.T) {
	d := testkit.NewDatabase(t)
	ctx := context.Background()
	var committed, rolledBack bool
	_ = db.InTx(ctx, d.App, func(tx *db.Tx) error {
		tx.AfterCommit(func() { committed = true })
		return nil
	})
	_ = db.InTx(ctx, d.App, func(tx *db.Tx) error {
		tx.AfterCommit(func() { rolledBack = true })
		return errors.New("abort")
	})
	if !committed || rolledBack {
		t.Fatalf("committed=%t rolledBack=%t", committed, rolledBack)
	}
}

func isPGCode(err error, code string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == code
}
