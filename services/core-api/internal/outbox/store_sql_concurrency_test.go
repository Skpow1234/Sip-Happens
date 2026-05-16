package outbox

import (
	"context"
	"sync"
	"testing"
	"time"

	"sip-happens/services/core-api/internal/testutil"
)

func TestSQLStore_ClaimPending_ConcurrentNoDuplicates(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	store := NewSQLStore()

	const total = 50
	if err := InsertPendingEvents(ctx, tdb.DB, "test.event", total); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	workers := []string{"w1", "w2", "w3", "w4"}

	var mu sync.Mutex
	claimed := map[string]string{} // id -> worker

	var wg sync.WaitGroup
	wg.Add(len(workers))
	for _, w := range workers {
		worker := w
		go func() {
			defer wg.Done()
			evts, err := store.ClaimPending(ctx, tdb.DB, now, total, worker)
			if err != nil {
				t.Errorf("claim failed worker=%s: %v", worker, err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, e := range evts {
				if prev, ok := claimed[e.ID]; ok {
					t.Errorf("duplicate claim id=%s prev=%s now=%s", e.ID, prev, worker)
					continue
				}
				claimed[e.ID] = worker
			}
		}()
	}
	wg.Wait()

	if len(claimed) != total {
		t.Fatalf("expected %d claimed events, got %d", total, len(claimed))
	}
}
