package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisSWRCacheHitAfterStore(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	cache := NewRedisSWRCache(rdb, "test", 2*time.Second, 4*time.Second)
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	h := cache.WrapGET(next, func(r *http.Request) string { return "k1" })

	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, req)
	if w1.Header().Get("x-cache") != "MISS" {
		t.Fatalf("first x-cache=%q", w1.Header().Get("x-cache"))
	}

	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/catalog", nil))
	if w2.Header().Get("x-cache") != "HIT" {
		t.Fatalf("second x-cache=%q", w2.Header().Get("x-cache"))
	}
	if calls != 1 {
		t.Fatalf("expected 1 upstream call, got %d", calls)
	}
	body, _ := io.ReadAll(w2.Result().Body)
	if string(body) != `{"ok":true}` {
		t.Fatalf("body=%s", body)
	}
}
