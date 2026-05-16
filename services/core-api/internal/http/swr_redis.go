package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisSWRStore struct {
	rdb    *redis.Client
	prefix string
}

type swrRedisPayload struct {
	Status      int    `json:"s"`
	ContentType string `json:"ct"`
	BodyB64     string `json:"b"`
	FreshUntil  int64  `json:"fu"`
	StaleUntil  int64  `json:"su"`
}

func newRedisSWRStore(rdb *redis.Client, namespace string) *redisSWRStore {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = "dev"
	}
	return &redisSWRStore{
		rdb:    rdb,
		prefix: fmt.Sprintf("sip:%s:swr:", ns),
	}
}

func (s *redisSWRStore) dataKey(key string) string {
	return s.prefix + key
}

func (s *redisSWRStore) lockKey(key string) string {
	return s.prefix + "lock:" + key
}

func (s *redisSWRStore) load(key string, now time.Time) (swrItem, swrLookup) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	raw, err := s.rdb.Get(ctx, s.dataKey(key)).Bytes()
	if err == redis.Nil || err != nil {
		return swrItem{}, swrLookupMiss
	}
	var p swrRedisPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return swrItem{}, swrLookupMiss
	}
	fresh := time.Unix(0, p.FreshUntil)
	stale := time.Unix(0, p.StaleUntil)
	if now.After(stale) {
		_ = s.rdb.Del(ctx, s.dataKey(key)).Err()
		return swrItem{}, swrLookupMiss
	}
	body, err := base64.StdEncoding.DecodeString(p.BodyB64)
	if err != nil {
		return swrItem{}, swrLookupMiss
	}
	it := swrItem{
		status:      p.Status,
		contentType: p.ContentType,
		body:        body,
		freshUntil:  fresh,
		staleUntil:  stale,
	}
	if now.Before(fresh) {
		return it, swrLookupFresh
	}
	return it, swrLookupStale
}

func (s *redisSWRStore) save(key string, it swrItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ttl := time.Until(it.staleUntil)
	if ttl <= 0 {
		return
	}
	p := swrRedisPayload{
		Status:      it.status,
		ContentType: it.contentType,
		BodyB64:     base64.StdEncoding.EncodeToString(it.body),
		FreshUntil:  it.freshUntil.UnixNano(),
		StaleUntil:  it.staleUntil.UnixNano(),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, s.dataKey(key), raw, ttl).Err()
}

func (s *redisSWRStore) remove(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = s.rdb.Del(ctx, s.dataKey(key), s.lockKey(key)).Err()
}

func (s *redisSWRStore) tryBeginRefresh(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ok, err := s.rdb.SetNX(ctx, s.lockKey(key), "1", 30*time.Second).Result()
	return err == nil && ok
}

func (s *redisSWRStore) endRefresh(key string, it swrItem, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = s.rdb.Del(ctx, s.lockKey(key)).Err()
	if ok {
		s.save(key, it)
	}
}
