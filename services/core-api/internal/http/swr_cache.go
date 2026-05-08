package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"time"
)

type SWRCache struct {
	FreshFor time.Duration
	StaleFor time.Duration

	mu    sync.Mutex
	items map[string]*swrItem
}

type swrItem struct {
	status      int
	contentType string
	body        []byte

	freshUntil time.Time
	staleUntil time.Time
	refreshing bool
}

func (c *SWRCache) WrapGET(next http.Handler, keyFn func(r *http.Request) string) http.Handler {
	if c.items == nil {
		c.items = map[string]*swrItem{}
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
		if hit, ok := c.tryServe(w, r, k, now, next); ok {
			_ = hit
			return
		}
		// Miss: compute synchronously.
		status, ct, body := computeResponse(next, r)
		if status == http.StatusOK && len(body) > 0 {
			c.store(k, status, ct, body, now)
		}
		writeRecorded(w, status, ct, body, "MISS")
	})
}

func (c *SWRCache) tryServe(w http.ResponseWriter, r *http.Request, key string, now time.Time, next http.Handler) (served bool, ok bool) {
	c.mu.Lock()
	it, exists := c.items[key]
	if exists && now.After(it.staleUntil) {
		delete(c.items, key)
		exists = false
	}
	if !exists {
		c.mu.Unlock()
		return false, false
	}

	if now.Before(it.freshUntil) {
		status, ct, body := it.status, it.contentType, append([]byte(nil), it.body...)
		c.mu.Unlock()
		writeRecorded(w, status, ct, body, "HIT")
		return true, true
	}

	// Stale-while-revalidate.
	if now.Before(it.staleUntil) {
		shouldRefresh := !it.refreshing
		if shouldRefresh {
			it.refreshing = true
		}
		status, ct, body := it.status, it.contentType, append([]byte(nil), it.body...)
		c.mu.Unlock()

		if shouldRefresh {
			go c.refresh(context.Background(), key, r, next)
		}
		writeRecorded(w, status, ct, body, "STALE")
		return true, true
	}

	// Too old.
	delete(c.items, key)
	c.mu.Unlock()
	return false, false
}

func (c *SWRCache) refresh(ctx context.Context, key string, r *http.Request, next http.Handler) {
	// Clone the request to avoid races on body/ctx. Catalog GET has no body.
	req := r.Clone(ctx)
	status, ct, body := computeResponse(next, req)

	c.mu.Lock()
	defer c.mu.Unlock()
	it, ok := c.items[key]
	if !ok {
		return
	}
	it.refreshing = false
	if status == http.StatusOK && len(body) > 0 {
		now := time.Now()
		it.status = status
		it.contentType = ct
		it.body = append([]byte(nil), body...)
		it.freshUntil = now.Add(c.FreshFor)
		it.staleUntil = now.Add(c.FreshFor + c.StaleFor)
	}
}

func (c *SWRCache) store(key string, status int, ct string, body []byte, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &swrItem{
		status:      status,
		contentType: ct,
		body:        append([]byte(nil), body...),
		freshUntil:  now.Add(c.FreshFor),
		staleUntil:  now.Add(c.FreshFor + c.StaleFor),
	}
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

