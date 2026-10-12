// Command mcctl is the MateCollab administration CLI used in development, deployment and
// operations: database bootstrap and migrations, storage initialisation, key generation,
// configuration checks and dependency diagnostics.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/rizwansammo/MateCollab/services/internal/apiserver"
	"github.com/rizwansammo/MateCollab/services/internal/app"
	"github.com/rizwansammo/MateCollab/services/internal/orgs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/apperr"
	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
	"github.com/rizwansammo/MateCollab/services/internal/platform/crypto"
	"github.com/rizwansammo/MateCollab/services/internal/platform/db"
	"github.com/rizwansammo/MateCollab/services/internal/platform/health"
	"github.com/rizwansammo/MateCollab/services/internal/platform/jobs"
	"github.com/rizwansammo/MateCollab/services/internal/platform/livekit"
	"github.com/rizwansammo/MateCollab/services/internal/platform/mailer"
	"github.com/rizwansammo/MateCollab/services/internal/platform/objectstore"
	"github.com/rizwansammo/MateCollab/services/internal/platform/redisx"
	"github.com/rizwansammo/MateCollab/services/internal/platform/telemetry"
)

const usage = `mcctl — MateCollab administration

Usage:
  mcctl prepare               bootstrap (when a superuser URL is set), migrate up, storage init
  mcctl db bootstrap          create/update database roles and the database (superuser URL required)
  mcctl migrate up            apply pending migrations (application schema, then job queue schema)
  mcctl migrate down          roll back the most recent application migration
  mcctl migrate status        list migrations and whether they are applied
  mcctl storage init          create the bucket and apply CORS/lifecycle rules where supported
  mcctl mail test --to ADDR   send a test email through the platform mailer
  mcctl org create --name NAME --slug SLUG --owner-email ADDR [--owner-name NAME] [--print-setup-link]
                              create an organisation; a new owner account is emailed a
                              set-your-password link
  mcctl keys generate         print a new encryption keyring entry
  mcctl config check          validate configuration for every binary
  mcctl doctor                check connectivity to every local dependency
  mcctl version               print the build version
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mcctl:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(out, usage)
		return nil
	}
	cmd := strings.Join(args[:min(2, len(args))], " ")
	switch {
	case args[0] == "version":
		fmt.Fprintln(out, app.BuildVersion())
		return nil
	case cmd == "keys generate":
		return keysGenerate(args[2:], out)
	}

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	logger := telemetry.NewLogger(config.LogConfig{Level: slog.LevelInfo, Format: "text"}, "mcctl", app.BuildVersion())

	switch cmd {
	case "prepare":
		return prepare(ctx, cfg, logger, out)
	case "db bootstrap":
		return bootstrap(ctx, cfg, logger)
	case "migrate up", "migrate down", "migrate status":
		return migrate(ctx, cfg, args[1], logger, out)
	case "storage init":
		return storageInit(ctx, cfg, out)
	case "mail test":
		return mailTest(ctx, cfg, args[2:], out)
	case "org create":
		return orgCreate(ctx, cfg, logger, args[2:], out)
	case "config check":
		return configCheck(cfg, out)
	case "doctor":
		return doctor(ctx, cfg, out)
	}
	fmt.Fprint(out, usage)
	return fmt.Errorf("unknown command %q", strings.Join(args, " "))
}

// prepare brings a deployment's database and bucket up to date with this build. Bootstrap is
// skipped when no superuser URL is configured (managed databases provision roles separately).
func prepare(ctx context.Context, cfg *config.Config, logger *slog.Logger, out io.Writer) error {
	if cfg.Database.BootstrapURL != "" {
		if err := bootstrap(ctx, cfg, logger); err != nil {
			return err
		}
	}
	if err := migrate(ctx, cfg, "up", logger, out); err != nil {
		return err
	}
	return storageInit(ctx, cfg, out)
}

func bootstrap(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	if err := cfg.Require(config.PartBootstrap); err != nil {
		return err
	}
	return db.Bootstrap(ctx, db.BootstrapOptions{SuperuserURL: cfg.Database.BootstrapURL, AppURL: cfg.Database.URL, MigratorURL: cfg.Database.MigrateURL}, logger)
}

func migrate(ctx context.Context, cfg *config.Config, action string, logger *slog.Logger, out io.Writer) error {
	if err := cfg.Require(config.PartMigrations); err != nil {
		return err
	}
	m, err := db.NewMigrator(ctx, cfg.Database.MigrateURL, cfg.Database.MigrationsDir, logger)
	if err != nil {
		return err
	}
	defer m.Close()
	switch action {
	case "up":
		return m.Up(ctx)
	case "down":
		return m.Down(ctx)
	default:
		statuses, err := m.Status(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "VERSION\tSTATE\tAPPLIED AT\tFILE")
		for _, s := range statuses {
			applied := ""
			if !s.AppliedAt.IsZero() {
				applied = s.AppliedAt.Format(time.RFC3339)
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", s.Source.Version, s.State, applied, baseName(s.Source.Path))
		}
		return tw.Flush()
	}
}

func storageInit(ctx context.Context, cfg *config.Config, out io.Writer) error {
	if err := cfg.Require(config.PartStorage); err != nil {
		return err
	}
	store, err := objectstore.NewS3(cfg.Storage)
	if err != nil {
		return err
	}
	res, err := store.Init(ctx, cfg.AllowedOrigins)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "bucket %q ready (created: %t)\nCORS: %s\nlifecycle: %s\n", cfg.Storage.Bucket, res.BucketCreated, res.CORS, res.Lifecycle)
	return nil
}

func mailTest(ctx context.Context, cfg *config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("mail test", flag.ContinueOnError)
	to := fs.String("to", "", "recipient address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		return errors.New("--to is required")
	}
	if err := cfg.Require(config.PartSMTP); err != nil {
		return err
	}
	m, err := mailer.New(cfg.SMTP)
	if err != nil {
		return err
	}
	msg := mailer.Message{
		To:      []string{*to},
		Subject: "MateCollab test email",
		Text:    "This is a test email from the MateCollab platform mailer.\n\nIf you can read this, outgoing email is configured correctly.\n",
		HTML:    "<p>This is a test email from the <strong>MateCollab</strong> platform mailer.</p><p>If you can read this, outgoing email is configured correctly.</p>",
	}
	if err := m.Send(ctx, msg); err != nil {
		return err
	}
	fmt.Fprintf(out, "test email sent to %s via %s:%d\n", *to, cfg.SMTP.Host, cfg.SMTP.Port)
	return nil
}

func orgCreate(ctx context.Context, cfg *config.Config, logger *slog.Logger, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("org create", flag.ContinueOnError)
	name := fs.String("name", "", "organisation name")
	slug := fs.String("slug", "", "URL name: 3–40 lowercase letters, numbers and hyphens")
	ownerEmail := fs.String("owner-email", "", "email address of the first Owner")
	ownerName := fs.String("owner-name", "", "display name, used when the owner account is created")
	printLink := fs.Bool("print-setup-link", false, "also print the single-use setup link (when email delivery is unavailable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" || *slug == "" || *ownerEmail == "" {
		return errors.New("--name, --slug and --owner-email are required")
	}
	if err := cfg.Require(config.PartHTTP, config.PartDatabase, config.PartRedis, config.PartCrypto); err != nil {
		return err
	}
	pool, err := db.NewPool(ctx, cfg.Database.URL, db.PoolOptions{ApplicationName: "mc-mcctl", MaxConns: 2})
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
	queue, err := jobs.NewInsertOnlyClient(pool, logger)
	if err != nil {
		return err
	}
	svc, err := apiserver.NewServices(apiserver.Deps{Config: cfg, Pool: pool, Redis: rdb, Jobs: queue, Logger: logger})
	if err != nil {
		return err
	}
	displayName := *ownerName
	if displayName == "" {
		displayName = strings.Split(*ownerEmail, "@")[0]
	}
	res, err := svc.Orgs.CreateWithOwner(ctx, *name, strings.ToLower(*slug), orgs.OwnerSpec{Email: *ownerEmail, DisplayName: displayName})
	if err != nil {
		if e, ok := apperr.As(err); ok && len(e.Fields) > 0 {
			for _, f := range e.Fields {
				fmt.Fprintf(out, "  %s: %s\n", f.Field, f.Message)
			}
		}
		return err
	}
	fmt.Fprintf(out, "organisation %q created\n  id:   %s\n  url:  %s/%s/home\n", res.Organisation.Name, res.Organisation.ID,
		strings.TrimRight(cfg.PublicURL.String(), "/"), res.Organisation.Slug)
	if res.NewAccount {
		fmt.Fprintf(out, "owner account created for %s; a set-your-password email was queued.\n", *ownerEmail)
		if *printLink {
			fmt.Fprintf(out, "  setup link (single use, 72 hours; treat it like a password): %s\n", res.SetupLink)
		}
	} else {
		fmt.Fprintf(out, "existing account %s is the Owner.\n", *ownerEmail)
	}
	return nil
}

func keysGenerate(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("keys generate", flag.ContinueOnError)
	kid := fs.String("kid", "k"+time.Now().UTC().Format("20060102"), "key id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s:%s\n", *kid, key)
	return nil
}

func configCheck(cfg *config.Config, out io.Writer) error {
	parts := map[string][]config.Part{
		"api":                        {config.PartHTTP, config.PartAdmin, config.PartDatabase, config.PartRedis, config.PartCrypto},
		"worker":                     {config.PartAdmin, config.PartDatabase, config.PartRedis, config.PartSMTP, config.PartCrypto},
		"mcctl migrate/storage/mail": {config.PartMigrations, config.PartStorage, config.PartSMTP},
		"platform services":          {config.PartLiveKit, config.PartCrypto},
	}
	var failed bool
	for _, name := range []string{"api", "worker", "mcctl migrate/storage/mail", "platform services"} {
		if err := cfg.Require(parts[name]...); err != nil {
			failed = true
			fmt.Fprintf(out, "✗ %s\n  %s\n", name, strings.ReplaceAll(err.Error(), "\n", "\n  "))
		} else {
			fmt.Fprintf(out, "✓ %s\n", name)
		}
	}
	if cfg.Crypto.Keyring != "" {
		if _, err := crypto.ParseKeyring(cfg.Crypto.Keyring, cfg.Crypto.ActiveKey); err != nil {
			failed = true
			fmt.Fprintf(out, "✗ encryption keyring: %v\n", err)
		}
	}
	if cfg.Crypto.HMACSecret != "" {
		if _, err := crypto.DeriveSubkeys(cfg.Crypto.HMACSecret); err != nil {
			failed = true
			fmt.Fprintf(out, "✗ %v\n", err)
		}
	}
	if failed {
		return errors.New("configuration is incomplete")
	}
	fmt.Fprintf(out, "configuration OK for environment %q\n", cfg.Env)
	return nil
}

func doctor(ctx context.Context, cfg *config.Config, out io.Writer) error {
	var checks []health.Check
	add := func(name string, probe func(context.Context) error) {
		checks = append(checks, health.Check{Name: name, Critical: true, Probe: probe})
	}
	add("postgres (runtime role, schema version)", func(ctx context.Context) error {
		pool, err := db.NewPool(ctx, cfg.Database.URL, db.PoolOptions{ApplicationName: "mc-doctor", MaxConns: 2})
		if err != nil {
			return err
		}
		defer pool.Close()
		return db.CheckSchema(ctx, pool, cfg.Database.MigrationsDir)
	})
	add("valkey", func(ctx context.Context) error {
		c, err := redisx.New(cfg.Redis.URL)
		if err != nil {
			return err
		}
		defer c.Close()
		return redisx.Ping(ctx, c)
	})
	add("object storage", func(ctx context.Context) error {
		s, err := objectstore.NewS3(cfg.Storage)
		if err != nil {
			return err
		}
		return s.Ping(ctx)
	})
	add("smtp", func(ctx context.Context) error {
		m, err := mailer.New(cfg.SMTP)
		if err != nil {
			return err
		}
		return m.Ping(ctx)
	})
	add("livekit", func(ctx context.Context) error { return livekit.Ping(ctx, cfg.LiveKit.APIURL) })

	rep := health.Run(ctx, checks, 15*time.Second)
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, r := range rep.Checks {
		mark := "✓"
		if r.Status != "ok" {
			mark = "✗"
		}
		fmt.Fprintf(tw, "%s\t%s\t%dms\t%s\n", mark, r.Name, r.Millis, r.Error)
	}
	_ = tw.Flush()
	if rep.Status != "ready" {
		return errors.New("one or more dependencies are unavailable")
	}
	return nil
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}
