package testutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
)

type TestDB struct {
	DB      *sql.DB
	Schema  string
	CloseFn func() error
}

// OpenMigrated opens Postgres using POSTGRES_DSN and an isolated schema with migrations applied.
func OpenMigrated(ctx context.Context) (*TestDB, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required for integration tests")
	}
	return openMigratedDSN(ctx, dsn, "")
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
