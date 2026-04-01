package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ecleangg/booky/internal/testutil"
	"gopkg.in/yaml.v3"
)

func TestNewInitializesApplication(t *testing.T) {
	repo, dsn := testutil.NewTestRepository(t)
	repo.Close()

	cfg := testutil.TestConfig()
	cfg.Postgres.DSN = dsn

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := copyMigrations(t, filepath.Join(root, "migrations")); err != nil {
		t.Fatalf("copy migrations: %v", err)
	}

	configBytes, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configPath := filepath.Join(root, "config", "booky.yaml")
	if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	application, err := New(context.Background(), configPath)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(application.Repo.Close)

	if application.Config.Postgres.DSN != dsn {
		t.Fatalf("unexpected dsn %q", application.Config.Postgres.DSN)
	}
	if application.HTTPServer == nil || application.Scheduler == nil || application.Filings == nil {
		t.Fatalf("application dependencies were not initialized: %#v", application)
	}
}

func copyMigrations(t *testing.T, dest string) error {
	t.Helper()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(testutil.RepoRoot(t), "migrations"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(testutil.RepoRoot(t), "migrations", entry.Name())
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dest, entry.Name()), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
