package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/lib/pq"

	"sip-happens/services/core-api/internal/migrate"
)

// OpenMigratedForTest opens an isolated migrated schema using POSTGRES_DSN when set,
// otherwise a disposable Postgres 16 container via testcontainers (requires Docker).
func OpenMigratedForTest(ctx context.Context, t *testing.T) (*TestDB, error) {
	t.Helper()
	if dsn := os.Getenv("POSTGRES_DSN"); dsn != "" {
		return openMigratedDSN(ctx, dsn, "")
	}
	return openMigratedTestcontainers(ctx, t)
}

func openMigratedTestcontainers(ctx context.Context, t *testing.T) (*TestDB, error) {
	t.Helper()

	pg, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("beer_shop"),
		postgres.WithUsername("beer"),
		postgres.WithPassword("beer"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(90*time.Second)),
	)
	if err != nil {
		return nil, fmt.Errorf("postgres testcontainer: %w", err)
	}

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = testcontainers.TerminateContainer(pg)
		return nil, err
	}

	tdb, err := openMigratedDSN(ctx, dsn, "")
	if err != nil {
		_ = testcontainers.TerminateContainer(pg)
		return nil, err
	}

	origClose := tdb.CloseFn
	tdb.CloseFn = func() error {
		_ = origClose()
		return testcontainers.TerminateContainer(pg)
	}
	return tdb, nil
}

func openMigratedDSN(ctx context.Context, dsn, schema string) (*TestDB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	if schema == "" {
		schema = "test_" + randomHex(6)
	}
	if _, err := db.ExecContext(ctx, `create schema if not exists `+schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `set search_path to `+schema); err != nil {
		_ = db.Close()
		return nil, err
	}

	root, err := FindRepoRoot()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	m := migrate.Migrator{
		DB:  db,
		FS:  os.DirFS(filepath.Join(root, "infra", "migrations")),
		Dir: ".",
	}
	if err := m.Up(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	tdb := &TestDB{DB: db, Schema: schema}
	tdb.CloseFn = func() error {
		_, _ = db.ExecContext(context.Background(), `drop schema if exists `+schema+` cascade`)
		return db.Close()
	}
	return tdb, nil
}
