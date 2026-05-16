// Command outbox-multi-node-smoke seeds and verifies outbox rows for multi-worker CI smoke.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"

	"sip-happens/services/core-api/internal/outbox"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: outbox-multi-node-smoke <seed|pending|verify> ...")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "outbox-multi-node-smoke: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	count := fs.Int("count", 50, "expected event count")
	runID := fs.String("run-id", "", "unique run id (required)")
	_ = fs.Parse(args)

	if *runID == "" {
		return fmt.Errorf("--run-id is required")
	}
	eventType := "test.multi_node." + *runID

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := openDB(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	switch cmd {
	case "seed":
		if *count <= 0 {
			return fmt.Errorf("--count must be > 0")
		}
		return outbox.InsertPendingEvents(ctx, db, eventType, *count)
	case "pending":
		n, err := outbox.CountOutboxByStatus(ctx, db, eventType, "PENDING")
		if err != nil {
			return err
		}
		fmt.Println(n)
		return nil
	case "verify":
		sent, err := outbox.CountOutboxByStatus(ctx, db, eventType, "SENT")
		if err != nil {
			return err
		}
		if sent != *count {
			return fmt.Errorf("expected %d SENT events, got %d", *count, sent)
		}
		pending, err := outbox.CountOutboxByStatus(ctx, db, eventType, "PENDING")
		if err != nil {
			return err
		}
		if pending != 0 {
			return fmt.Errorf("expected 0 PENDING, got %d", pending)
		}
		fmt.Printf("verified %d SENT events for %s\n", sent, eventType)
		return nil
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func openDB(ctx context.Context) (*sql.DB, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
