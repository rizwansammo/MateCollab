// Package config loads and validates MateCollab configuration from the environment.
//
// Configuration only comes from environment variables. Any variable can instead be read from a
// file by setting NAME_FILE (Docker secrets). Organisation-specific behaviour never lives here.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Env is the deployment environment.
type Env string

// Supported environments.
const (
	EnvLocal      Env = "local"
	EnvCI         Env = "ci"
	EnvStaging    Env = "staging"
	EnvProduction Env = "production"
)

// Config is the complete process configuration.
type Config struct {
	Env            Env
	PublicURL      *url.URL
	AllowedOrigins []string
	HTTPAddr       string
	AdminAddr      string
	TrustedProxies []netip.Prefix

	Log       LogConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Storage   StorageConfig
	LiveKit   LiveKitConfig
	SMTP      SMTPConfig
	Crypto    CryptoConfig
	Scanner   ScannerConfig
	Telemetry TelemetryConfig
	Features  FeaturesConfig
	Limits    LimitsConfig
	Outbound  OutboundConfig
}

// LogConfig controls structured logging.
type LogConfig struct {
	Level  slog.Level
	Format string // "json" or "text"
}

// DatabaseConfig holds PostgreSQL connection settings for each database role.
type DatabaseConfig struct {
	URL           string // runtime role mc_app
	MigrateURL    string // migration role mc_migrator (mcctl only)
	BootstrapURL  string // superuser, local/bootstrap only (mcctl db bootstrap)
	MigrationsDir string
	MaxConns      int32
}

// RedisConfig points at the Redis-compatible server (Valkey).
type RedisConfig struct {
	URL string
}

// StorageConfig configures the S3-compatible object store.
type StorageConfig struct {
	Endpoint        string // reachable by the API and worker
	PublicEndpoint  string // reachable by browsers; used only to sign URLs
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

// LiveKitConfig configures the media server integration.
type LiveKitConfig struct {
	APIURL    string
	PublicURL string
	APIKey    string
	APISecret string
}

// SMTPConfig configures the platform mailer.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLS      string // "none", "starttls" or "tls"
}

// CryptoConfig holds key material for application-level encryption and derived keys.
type CryptoConfig struct {
	Keyring    string // "kid:base64key,kid2:base64key"
	ActiveKey  string
	HMACSecret string
}

// ScannerConfig selects the malware scanner.
type ScannerConfig struct {
	Mode       string // "clamav" or "noop"
	ClamAVAddr string
}

// TelemetryConfig configures OpenTelemetry export.
type TelemetryConfig struct {
	OTLPEndpoint string
}

// FeaturesConfig holds deployment-level feature switches.
type FeaturesConfig struct {
	AllowOrgSignup bool
}

// LimitsConfig holds platform-wide hard limits (organisations can only lower them).
type LimitsConfig struct {
	MaxUploadMB int
}

// OutboundConfig controls SSRF protection for administrator-configured endpoints.
type OutboundConfig struct {
	AllowPrivateCIDRs []netip.Prefix
}

// Part names a group of settings a binary needs; Require validates only the parts in use.
type Part int

// Configuration parts.
const (
	PartHTTP Part = iota
	PartAdmin
	PartDatabase
	PartMigrations
	PartBootstrap
	PartRedis
	PartStorage
	PartLiveKit
	PartSMTP
	PartCrypto
)

// Getenv looks up an environment variable; os.Getenv in production, a map in tests.
type Getenv func(string) string

// Load reads the configuration. Malformed values are reported together; missing values are
// only reported by Require so each binary validates what it actually uses.
func Load(getenv Getenv) (*Config, error) {
	l := loader{getenv: getenv}
	c := &Config{
		Env:            Env(l.str("MC_ENV", string(EnvLocal))),
		AllowedOrigins: l.list("MC_ALLOWED_ORIGINS"),
		HTTPAddr:       l.str("MC_HTTP_ADDR", ":8081"),
		AdminAddr:      l.str("MC_ADMIN_ADDR", ":8091"),
		TrustedProxies: l.prefixes("MC_TRUSTED_PROXIES"),
		Log: LogConfig{
			Level:  l.level("MC_LOG_LEVEL", slog.LevelInfo),
			Format: l.oneOf("MC_LOG_FORMAT", "json", "json", "text"),
		},
		Database: DatabaseConfig{
			URL:           l.secret("MC_DATABASE_URL"),
			MigrateURL:    l.secret("MC_MIGRATE_DATABASE_URL"),
			BootstrapURL:  l.secret("MC_DATABASE_BOOTSTRAP_URL"),
			MigrationsDir: l.str("MC_MIGRATIONS_DIR", ""),
			MaxConns:      int32(l.int("MC_DB_MAX_CONNS", 20)),
		},
		Redis: RedisConfig{URL: l.secret("MC_REDIS_URL")},
		Storage: StorageConfig{
			Endpoint:        l.str("MC_S3_ENDPOINT", ""),
			PublicEndpoint:  l.str("MC_S3_PUBLIC_ENDPOINT", ""),
			Region:          l.str("MC_S3_REGION", "us-east-1"),
			Bucket:          l.str("MC_S3_BUCKET", "matecollab"),
			AccessKeyID:     l.secret("MC_S3_ACCESS_KEY_ID"),
			SecretAccessKey: l.secret("MC_S3_SECRET_ACCESS_KEY"),
			ForcePathStyle:  l.bool("MC_S3_FORCE_PATH_STYLE", true),
		},
		LiveKit: LiveKitConfig{
			APIURL:    l.str("MC_LIVEKIT_API_URL", ""),
			PublicURL: l.str("MC_LIVEKIT_PUBLIC_URL", ""),
			APIKey:    l.secret("MC_LIVEKIT_API_KEY"),
			APISecret: l.secret("MC_LIVEKIT_API_SECRET"),
		},
		SMTP: SMTPConfig{
			Host:     l.str("MC_SMTP_HOST", ""),
			Port:     l.int("MC_SMTP_PORT", 587),
			Username: l.secret("MC_SMTP_USERNAME"),
			Password: l.secret("MC_SMTP_PASSWORD"),
			From:     l.str("MC_SMTP_FROM", ""),
			TLS:      l.oneOf("MC_SMTP_TLS", "starttls", "none", "starttls", "tls"),
		},
		Crypto: CryptoConfig{
			Keyring:    l.secret("MC_ENCRYPTION_KEYRING"),
			ActiveKey:  l.str("MC_ENCRYPTION_ACTIVE_KEY", ""),
			HMACSecret: l.secret("MC_HMAC_SECRET"),
		},
		Scanner: ScannerConfig{
			Mode:       l.oneOf("MC_SCANNER", "clamav", "clamav", "noop"),
			ClamAVAddr: l.str("MC_CLAMAV_ADDR", "localhost:3310"),
		},
		Telemetry: TelemetryConfig{OTLPEndpoint: l.str("MC_OTEL_EXPORTER_OTLP_ENDPOINT", "")},
		Features:  FeaturesConfig{AllowOrgSignup: l.bool("MC_ALLOW_ORG_SIGNUP", false)},
		Limits:    LimitsConfig{MaxUploadMB: l.int("MC_MAX_UPLOAD_MB", 100)},
		Outbound:  OutboundConfig{AllowPrivateCIDRs: l.prefixes("MC_OUTBOUND_ALLOW_PRIVATE_CIDRS")},
	}

	switch c.Env {
	case EnvLocal, EnvCI, EnvStaging, EnvProduction:
	default:
		l.fail("MC_ENV", "must be one of local, ci, staging, production")
	}
	if raw := l.str("MC_PUBLIC_URL", "http://localhost:8080"); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			l.fail("MC_PUBLIC_URL", "must be an absolute http(s) URL")
		} else {
			c.PublicURL = u
		}
	}
	if len(c.AllowedOrigins) == 0 && c.PublicURL != nil {
		c.AllowedOrigins = []string{c.PublicURL.Scheme + "://" + c.PublicURL.Host}
	}
	if c.Storage.PublicEndpoint == "" {
		c.Storage.PublicEndpoint = c.Storage.Endpoint
	}
	if c.Database.MigrationsDir == "" {
		c.Database.MigrationsDir = discoverMigrationsDir()
	}
	if c.Limits.MaxUploadMB < 1 || c.Limits.MaxUploadMB > 1024 {
		l.fail("MC_MAX_UPLOAD_MB", "must be between 1 and 1024")
	}
	if c.Database.MaxConns < 2 {
		l.fail("MC_DB_MAX_CONNS", "must be at least 2")
	}

	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	return c, nil
}

// Require reports every missing or unsafe setting for the given parts.
func (c *Config) Require(parts ...Part) error {
	var errs []error
	need := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", name))
		}
	}
	for _, p := range parts {
		switch p {
		case PartHTTP:
			need("MC_HTTP_ADDR", c.HTTPAddr)
			if c.PublicURL == nil {
				errs = append(errs, errors.New("MC_PUBLIC_URL is required"))
			}
		case PartAdmin:
			need("MC_ADMIN_ADDR", c.AdminAddr)
		case PartDatabase:
			need("MC_DATABASE_URL", c.Database.URL)
		case PartMigrations:
			need("MC_MIGRATE_DATABASE_URL", c.Database.MigrateURL)
			need("MC_MIGRATIONS_DIR (or a db/migrations directory above the working directory)", c.Database.MigrationsDir)
		case PartBootstrap:
			need("MC_DATABASE_BOOTSTRAP_URL", c.Database.BootstrapURL)
			need("MC_DATABASE_URL", c.Database.URL)
			need("MC_MIGRATE_DATABASE_URL", c.Database.MigrateURL)
		case PartRedis:
			need("MC_REDIS_URL", c.Redis.URL)
		case PartStorage:
			need("MC_S3_ENDPOINT", c.Storage.Endpoint)
			need("MC_S3_BUCKET", c.Storage.Bucket)
			need("MC_S3_ACCESS_KEY_ID", c.Storage.AccessKeyID)
			need("MC_S3_SECRET_ACCESS_KEY", c.Storage.SecretAccessKey)
		case PartLiveKit:
			need("MC_LIVEKIT_API_URL", c.LiveKit.APIURL)
			need("MC_LIVEKIT_PUBLIC_URL", c.LiveKit.PublicURL)
			need("MC_LIVEKIT_API_KEY", c.LiveKit.APIKey)
			need("MC_LIVEKIT_API_SECRET", c.LiveKit.APISecret)
		case PartSMTP:
			need("MC_SMTP_HOST", c.SMTP.Host)
			need("MC_SMTP_FROM", c.SMTP.From)
		case PartCrypto:
			need("MC_ENCRYPTION_KEYRING", c.Crypto.Keyring)
			need("MC_ENCRYPTION_ACTIVE_KEY", c.Crypto.ActiveKey)
			need("MC_HMAC_SECRET", c.Crypto.HMACSecret)
			if c.Crypto.HMACSecret != "" && len(c.Crypto.HMACSecret) < 32 {
				errs = append(errs, errors.New("MC_HMAC_SECRET must be at least 32 characters"))
			}
		}
	}
	if c.Env == EnvProduction {
		if c.PublicURL != nil && c.PublicURL.Scheme != "https" {
			errs = append(errs, errors.New("MC_PUBLIC_URL must use https in production"))
		}
		if c.Scanner.Mode != "clamav" {
			errs = append(errs, errors.New("MC_SCANNER must be clamav in production (uploads fail closed)"))
		}
		if c.SMTP.Host != "" && c.SMTP.TLS == "none" {
			errs = append(errs, errors.New("MC_SMTP_TLS=none is not allowed in production"))
		}
	}
	return errors.Join(errs...)
}

// IsLocal reports whether this is a developer machine.
func (c *Config) IsLocal() bool { return c.Env == EnvLocal }

type loader struct {
	getenv Getenv
	errs   []error
}

func (l *loader) fail(name, msg string) { l.errs = append(l.errs, fmt.Errorf("%s %s", name, msg)) }

// raw returns NAME, or the contents of the file named by NAME_FILE.
func (l *loader) raw(name string) string {
	if v := l.getenv(name); v != "" {
		return v
	}
	if path := l.getenv(name + "_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			l.fail(name+"_FILE", "cannot be read: "+err.Error())
			return ""
		}
		return strings.TrimRight(string(b), "\r\n")
	}
	return ""
}

func (l *loader) str(name, def string) string {
	if v := strings.TrimSpace(l.raw(name)); v != "" {
		return v
	}
	return def
}

func (l *loader) secret(name string) string { return l.raw(name) }

func (l *loader) int(name string, def int) int {
	v := l.str(name, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.fail(name, "must be an integer")
		return def
	}
	return n
}

func (l *loader) bool(name string, def bool) bool {
	v := l.str(name, "")
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.fail(name, "must be true or false")
		return def
	}
	return b
}

func (l *loader) oneOf(name, def string, allowed ...string) string {
	v := strings.ToLower(l.str(name, def))
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	l.fail(name, "must be one of "+strings.Join(allowed, ", "))
	return def
}

func (l *loader) list(name string) []string {
	var out []string
	for _, p := range strings.Split(l.str(name, ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (l *loader) prefixes(name string) []netip.Prefix {
	var out []netip.Prefix
	for _, p := range l.list(name) {
		pfx, err := netip.ParsePrefix(p)
		if err != nil {
			l.fail(name, fmt.Sprintf("contains an invalid CIDR %q", p))
			continue
		}
		out = append(out, pfx.Masked())
	}
	return out
}

func (l *loader) level(name string, def slog.Level) slog.Level {
	v := l.str(name, "")
	if v == "" {
		return def
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(v)); err != nil {
		l.fail(name, "must be debug, info, warn or error")
		return def
	}
	return lvl
}

// discoverMigrationsDir finds db/migrations above the working directory (developer convenience;
// containers set MC_MIGRATIONS_DIR explicitly).
func discoverMigrationsDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for range 6 {
		candidate := filepath.Join(dir, "db", "migrations")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
