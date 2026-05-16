package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/testutil"
)

// Two independent DB connections dispatch in parallel (multi-node claim semantics).
func TestMultiNodeOutboxDispatch_TwoWorkersNoDuplicates(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	const (
		total     = 40
		eventType = "test.multi_node.dispatch"
	)
	if err := InsertPendingEvents(ctx, tdb.DB, eventType, total); err != nil {
		t.Fatal(err)
	}

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN not set")
	}
	openConn := func() *sql.DB {
		c, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		if _, err := c.ExecContext(ctx, `set search_path to `+tdb.Schema); err != nil {
			t.Fatalf("search_path: %v", err)
		}
		return c
	}

	dbA := openConn()
	defer dbA.Close()
	dbB := openConn()
	defer dbB.Close()

	track := &publishTracker{}
	now := time.Now()

	runWorker := func(db *sql.DB, lockedBy string) {
		d := &Dispatcher{
			Store:     NewSQLStore(),
			Publisher: track,
			LockedBy:  lockedBy,
			Batch:     25,
		}
		for round := 0; round < 8; round++ {
			n, err := d.RunOnce(ctx, db, now)
			if err != nil {
				t.Errorf("worker %s: %v", lockedBy, err)
				return
			}
			if n == 0 {
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runWorker(dbA, "node-a")
	}()
	go func() {
		defer wg.Done()
		runWorker(dbB, "node-b")
	}()
	wg.Wait()

	if got := track.len(); got != total {
		t.Fatalf("expected %d published events, got %d", total, got)
	}

	sent, err := CountOutboxByStatus(ctx, tdb.DB, eventType, "SENT")
	if err != nil {
		t.Fatal(err)
	}
	if sent != total {
		t.Fatalf("expected %d SENT rows, got %d", total, sent)
	}
}

type publishTracker struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func (p *publishTracker) Publish(_ context.Context, e Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen == nil {
		p.seen = make(map[string]struct{})
	}
	if _, ok := p.seen[e.ID]; ok {
		return fmt.Errorf("duplicate publish id=%s", e.ID)
	}
	p.seen[e.ID] = struct{}{}
	return nil
}

func (p *publishTracker) len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.seen)
}
