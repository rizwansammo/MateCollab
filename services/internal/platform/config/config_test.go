package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func env(m map[string]string) Getenv { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Env != EnvLocal || c.HTTPAddr != ":8081" || c.AdminAddr != ":8091" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if got := c.AllowedOrigins; len(got) != 1 || got[0] != "http://localhost:8080" {
		t.Fatalf("allowed origins default from public URL, got %v", got)
	}
	if c.Scanner.Mode != "clamav" {
		t.Fatalf("scanner must default to clamav (fail closed), got %q", c.Scanner.Mode)
	}
}

func TestLoadReportsAllMalformedValues(t *testing.T) {
	_, err := Load(env(map[string]string{
		"MC_ENV":             "prod",
		"MC_DB_MAX_CONNS":    "lots",
		"MC_TRUSTED_PROXIES": "10.0.0.0/8,not-a-cidr",
		"MC_LOG_FORMAT":      "xml",
	}))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"MC_ENV", "MC_DB_MAX_CONNS", "MC_TRUSTED_PROXIES", "MC_LOG_FORMAT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s: %v", want, err)
		}
	}
}

func TestSecretFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("postgres://x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(env(map[string]string{"MC_DATABASE_URL_FILE": path}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Database.URL != "postgres://x" {
		t.Fatalf("got %q", c.Database.URL)
	}
}

func TestRequireListsMissingParts(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Require(PartDatabase, PartRedis, PartCrypto)
	for _, want := range []string{"MC_DATABASE_URL", "MC_REDIS_URL", "MC_ENCRYPTION_KEYRING", "MC_HMAC_SECRET"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Require should report %s, got %v", want, err)
		}
	}
}

func TestProductionSafetyChecks(t *testing.T) {
	c, err := Load(env(map[string]string{
		"MC_ENV":        "production",
		"MC_PUBLIC_URL": "http://example.test",
		"MC_SCANNER":    "noop",
		"MC_SMTP_HOST":  "smtp.example.test",
		"MC_SMTP_TLS":   "none",
	}))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Require()
	for _, want := range []string{"https", "MC_SCANNER", "MC_SMTP_TLS"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("production check should mention %s, got %v", want, err)
		}
	}
}
