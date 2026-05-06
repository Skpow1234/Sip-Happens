package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

type Migrator struct {
	DB *sql.DB
	FS fs.FS
	Dir string
}

func (m *Migrator) Up(ctx context.Context) error {
	if m.DB == nil {
		return fmt.Errorf("missing db")
	}
	if m.FS == nil {
		return fmt.Errorf("missing fs")
	}
	if m.Dir == "" {
		m.Dir = "."
	}

	if _, err := m.DB.ExecContext(ctx, `
create table if not exists schema_migrations (
  version text primary key,
  applied_at timestamptz not null default now()
)
`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(m.FS, m.Dir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")

		var exists int
		if err := m.DB.QueryRowContext(ctx, `select 1 from schema_migrations where version=$1`, version).Scan(&exists); err == nil {
			continue
		} else if err != sql.ErrNoRows {
			return err
		}

		migrationPath := name
		if m.Dir != "." && m.Dir != "" {
			migrationPath = path.Join(m.Dir, name)
		}
		b, err := fs.ReadFile(m.FS, migrationPath)
		if err != nil {
			return err
		}

		tx, err := m.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(b)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `insert into schema_migrations (version) values ($1)`, version); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

