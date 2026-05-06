package testutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"sip-happens/services/core-api/internal/migrate"
)

type TestDB struct {
	DB       *sql.DB
	Schema   string
	CloseFn  func() error
}

func OpenMigrated(ctx context.Context) (*TestDB, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required for integration tests")
	}

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

	schema := "test_" + randomHex(6)
	if _, err := db.ExecContext(ctx, `create schema if not exists `+schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, `set search_path to `+schema); err != nil {
		_ = db.Close()
		return nil, err
	}

	m := migrate.Migrator{
		DB:  db,
		FS:  os.DirFS("infra/migrations"),
		Dir: ".",
	}
	if err := m.Up(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	tdb := &TestDB{
		DB:     db,
		Schema: schema,
	}
	tdb.CloseFn = func() error {
		_, _ = db.ExecContext(context.Background(), `drop schema if exists `+schema+` cascade`)
		return db.Close()
	}
	return tdb, nil
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

