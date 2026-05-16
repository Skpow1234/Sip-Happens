package http

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisTokenBucketLimiterShared(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	lim := &RedisTokenBucketLimiter{
		Client:     rdb,
		KeyPrefix:  "test:rl",
		Capacity:   2,
		RefillRate: 0,
		TTL:        time.Minute,
	}
	now := time.Now()
	if !lim.Allow("user", now) {
		t.Fatal("expected first allow")
	}
	if !lim.Allow("user", now) {
		t.Fatal("expected second allow")
	}
	if lim.Allow("user", now) {
		t.Fatal("expected third deny")
	}
}
