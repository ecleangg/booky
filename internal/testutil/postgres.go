package testutil

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ecleangg/booky/internal/config"
	"github.com/ecleangg/booky/internal/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func NewTestRepository(t testing.TB) (*store.Repository, string) {
	t.Helper()

	baseDSN := os.Getenv("BOOKY_TEST_POSTGRES_DSN")
	if strings.TrimSpace(baseDSN) == "" {
		t.Skip("BOOKY_TEST_POSTGRES_DSN is not set")
	}

	schema := "booky_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	adminConn, err := pgx.Connect(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("connect postgres admin: %v", err)
	}
	defer adminConn.Close(context.Background())

	if _, err := adminConn.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create schema %s: %v", schema, err)
	}

	dsn := withSearchPath(baseDSN, schema)
	repo, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	migrationsDir := filepath.Join(RepoRoot(t), "migrations")
	if err := repo.RunMigrations(context.Background(), migrationsDir); err != nil {
		repo.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		repo.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, baseDSN)
		if err != nil {
			return
		}
		defer conn.Close(ctx)
		_, _ = conn.Exec(ctx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	return repo, dsn
}

func withSearchPath(dsn, schema string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	values := parsed.Query()
	values.Set("search_path", schema)
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func NewTestConfigWithDSN(t testing.TB) (config ConfigWithDSN) {
	t.Helper()
	repo, dsn := NewTestRepository(t)
	_ = repo
	cfg := TestConfig()
	cfg.Postgres.DSN = dsn
	return ConfigWithDSN{Config: cfg, DSN: dsn}
}

type ConfigWithDSN struct {
	Config config.Config
	DSN    string
}
