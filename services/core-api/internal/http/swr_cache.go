package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type SWRCache struct {
	FreshFor time.Duration
	StaleFor time.Duration

	store swrStore
}

type swrItem struct {
	status      int
	contentType string
	body        []byte

	freshUntil time.Time
	staleUntil time.Time
	refreshing bool
}

// NewSWRCache returns an in-memory stale-while-revalidate cache.
func NewSWRCache(fresh, stale time.Duration) *SWRCache {
	c := &SWRCache{FreshFor: fresh, StaleFor: stale}
	c.store = newMemorySWRStore()
	return c
}

// NewRedisSWRCache uses Redis as the backing store (shared across core-api replicas).
func NewRedisSWRCache(rdb *redis.Client, namespace string, fresh, stale time.Duration) *SWRCache {
	c := &SWRCache{FreshFor: fresh, StaleFor: stale}
	c.store = newRedisSWRStore(rdb, namespace)
	return c
}

func (c *SWRCache) WrapGET(next http.Handler, keyFn func(r *http.Request) string) http.Handler {
	if c.store == nil {
		c.store = newMemorySWRStore()
	}
	if c.FreshFor <= 0 {
		c.FreshFor = 15 * time.Second
	}
	if c.StaleFor <= 0 {
		c.StaleFor = 45 * time.Second
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		k := strings.TrimSpace(keyFn(r))
		if k == "" {
			next.ServeHTTP(w, r)
			return
		}

		now := time.Now()
		if c.tryServe(w, r, k, now, next) {
			return
		}
		status, ct, body := computeResponse(next, r)
		if status == http.StatusOK && len(body) > 0 {
			c.store.save(k, swrItem{
				status:      status,
				contentType: ct,
				body:        append([]byte(nil), body...),
				freshUntil:  now.Add(c.FreshFor),
				staleUntil:  now.Add(c.FreshFor + c.StaleFor),
			})
		}
		writeRecorded(w, status, ct, body, "MISS")
	})
}

func (c *SWRCache) tryServe(w http.ResponseWriter, r *http.Request, key string, now time.Time, next http.Handler) bool {
	it, kind := c.store.load(key, now)
	switch kind {
	case swrLookupFresh:
		writeRecorded(w, it.status, it.contentType, it.body, "HIT")
		return true
	case swrLookupStale:
		if c.store.tryBeginRefresh(key) {
			go c.refresh(context.Background(), key, r, next)
		}
		writeRecorded(w, it.status, it.contentType, it.body, "STALE")
		return true
	default:
		return false
	}
}

func (c *SWRCache) refresh(ctx context.Context, key string, r *http.Request, next http.Handler) {
	req := r.Clone(ctx)
	status, ct, body := computeResponse(next, req)

	ok := status == http.StatusOK && len(body) > 0
	var it swrItem
	if ok {
		now := time.Now()
		it = swrItem{
			status:      status,
			contentType: ct,
			body:        append([]byte(nil), body...),
			freshUntil:  now.Add(c.FreshFor),
			staleUntil:  now.Add(c.FreshFor + c.StaleFor),
		}
	}
	c.store.endRefresh(key, it, ok)
}

func computeResponse(next http.Handler, r *http.Request) (status int, contentType string, body []byte) {
	rr := httptest.NewRecorder()
	next.ServeHTTP(rr, r)
	res := rr.Result()
	defer res.Body.Close()
	b := rr.Body.Bytes()
	ct := res.Header.Get("content-type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	return res.StatusCode, ct, b
}

func writeRecorded(w http.ResponseWriter, status int, contentType string, body []byte, cacheStatus string) {
	if strings.TrimSpace(contentType) != "" {
		w.Header().Set("content-type", contentType)
	}
	if cacheStatus != "" {
		w.Header().Set("x-cache", cacheStatus)
	}
	w.WriteHeader(status)
	if len(body) > 0 {
		_, _ = w.Write(body)
	}
}

func canonicalQuery(r *http.Request, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	var parts []string
	q := r.URL.Query()
	for _, k := range keys {
		v := strings.TrimSpace(q.Get(k))
		if v == "" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "&")
}
