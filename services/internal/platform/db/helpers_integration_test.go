//go:build integration

package db_test

import (
	"testing"

	"github.com/rizwansammo/MateCollab/services/internal/platform/config"
)

func migrationsDir(t *testing.T) string {
	t.Helper()
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.MigrationsDir == "" {
		t.Fatal("db/migrations not found")
	}
	return cfg.Database.MigrationsDir
}
