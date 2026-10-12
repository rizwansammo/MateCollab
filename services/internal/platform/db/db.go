// Package db provides the PostgreSQL pool, transactions with tenant context, database
// bootstrap and migrations.
package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rizwansammo/MateCollab/services/internal/platform/tenant"
)

// OwnerRole owns every schema object. Migrations run as the migrator role with SET ROLE to it.
const OwnerRole = "mc_owner"

// PoolOptions tunes a connection pool.
type PoolOptions struct {
	ApplicationName  string
	MaxConns         int32
	StatementTimeout time.Duration // 0 = server default
	SetRole          string        // optional SET ROLE after connecting (migrations)
}

// NewPool opens a pgx pool and verifies connectivity.
func NewPool(ctx context.Context, url string, o PoolOptions) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: invalid connection URL: %w", redact(err))
	}
	if o.MaxConns > 0 {
		cfg.MaxConns = o.MaxConns
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	rp := cfg.ConnConfig.RuntimeParams
	if o.ApplicationName != "" {
		rp["application_name"] = o.ApplicationName
	}
	if o.StatementTimeout > 0 {
		rp["statement_timeout"] = strconv.FormatInt(o.StatementTimeout.Milliseconds(), 10)
	}
	rp["idle_in_transaction_session_timeout"] = "15000"
	if o.SetRole != "" {
		role := o.SetRole
		cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
			return err
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: cannot reach PostgreSQL: %w", err)
	}
	return pool, nil
}

// redact hides connection-string contents from parse errors.
func redact(error) error { return fmt.Errorf("connection string could not be parsed") }

// Tx is a transaction that can schedule work to run only after a successful commit
// (realtime publishes must never precede the commit — REALTIME.md §5).
type Tx struct {
	pgx.Tx
	afterCommit []func()
}

// AfterCommit registers fn to run after the transaction commits. It never runs on rollback.
func (t *Tx) AfterCommit(fn func()) { t.afterCommit = append(t.afterCommit, fn) }

// InTx runs fn in a transaction without tenant context (identity and platform tables only).
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(*Tx) error) error {
	return run(ctx, pool, nil, fn)
}

// InTenantTx runs fn in a transaction whose tenant settings are transaction-local, so they can
// never leak to the next user of a pooled connection.
func InTenantTx(ctx context.Context, pool *pgxpool.Pool, scope tenant.Scope, fn func(*Tx) error) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	return run(ctx, pool, &scope, fn)
}

// InUserTx runs fn with only the user context set (mc.user_id): the user's own identity rows
// and their memberships are visible, no organisation's tenant data is.
func InUserTx(ctx context.Context, pool *pgxpool.Pool, userID tenant.UserID, fn func(*Tx) error) error {
	if userID.IsZero() {
		return errors.New("db: user context requires a user ID")
	}
	return InTx(ctx, pool, func(tx *Tx) error {
		if err := SetScope(ctx, tx, tenant.Scope{UserID: userID}); err != nil {
			return err
		}
		return fn(tx)
	})
}

// SetScope sets (or changes) the transaction-local tenant settings on an open transaction, for
// flows that learn the user or organisation part-way through (token redemption, invitation
// acceptance). Empty parts are cleared.
func SetScope(ctx context.Context, tx *Tx, s tenant.Scope) error {
	org, user := "", ""
	if !s.OrgID.IsZero() {
		org = s.OrgID.String()
	}
	if !s.UserID.IsZero() {
		user = s.UserID.String()
	}
	_, err := tx.Exec(ctx, "SELECT set_config('mc.org_id', $1, true), set_config('mc.user_id', $2, true)", org, user)
	return err
}

func run(ctx context.Context, pool *pgxpool.Pool, scope *tenant.Scope, fn func(*Tx) error) (err error) {
	ptx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	tx := &Tx{Tx: ptx}
	defer func() {
		if p := recover(); p != nil {
			_ = ptx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			_ = ptx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if scope != nil {
		userID := ""
		if !scope.UserID.IsZero() {
			userID = scope.UserID.String()
		}
		if _, err = ptx.Exec(ctx, "SELECT set_config('mc.org_id', $1, true), set_config('mc.user_id', $2, true)",
			scope.OrgID.String(), userID); err != nil {
			return err
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	if err = ptx.Commit(ctx); err != nil {
		return err
	}
	for _, f := range tx.afterCommit {
		f()
	}
	return nil
}
