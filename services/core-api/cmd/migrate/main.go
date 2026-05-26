package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"

	"sip-happens/pkg/config"
	"sip-happens/services/core-api/internal/migrate"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "up" {
		log.Fatal("usage: core-migrate up")
	}
	if err := runMigrateUp(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func runMigrateUp(ctx context.Context) error {
	cfg, err := config.LoadCoreAPI()
	if err != nil {
		return err
	}
	sqlDB, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}

	m := migrate.Migrator{
		DB:  sqlDB,
		FS:  os.DirFS(cfg.MigrationsPath),
		Dir: ".",
	}
	return m.Up(ctx)
}
