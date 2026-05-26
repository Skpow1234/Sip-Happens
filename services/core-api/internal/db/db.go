package db

import (
	"context"
	"database/sql"
)

type DB struct {
	SQL *sql.DB
}

func New(sqlDB *sql.DB) *DB {
	return &DB{SQL: sqlDB}
}

func (d *DB) WithTx(ctx context.Context, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
	tx, err := d.SQL.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
