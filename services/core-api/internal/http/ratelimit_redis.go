package http

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisTokenBucketLimiter implements a shared token bucket using Redis + Lua.
type RedisTokenBucketLimiter struct {
	Client     *redis.Client
	KeyPrefix  string
	Capacity   float64
	RefillRate float64 // tokens per second
	TTL        time.Duration
}

var tokenBucketLua = redis.NewScript(`
local tokens_key = KEYS[1]
local ts_key = KEYS[2]
local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local rate = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])

local tokens = tonumber(redis.call('GET', tokens_key))
local last = tonumber(redis.call('GET', ts_key))
if tokens == nil then
  tokens = capacity
end
if last == nil then
  last = now
end
if tokens ~= nil then
  local delta = now - last
  if delta > 0 then
    tokens = math.min(capacity, tokens + delta * rate)
    last = now
  end
end
if tokens < 1 then
  redis.call('SET', tokens_key, tokens, 'EX', ttl)
  redis.call('SET', ts_key, last, 'EX', ttl)
  return 0
end
tokens = tokens - 1
redis.call('SET', tokens_key, tokens, 'EX', ttl)
redis.call('SET', ts_key, last, 'EX', ttl)
return 1
`)

func (l *RedisTokenBucketLimiter) Allow(key string, now time.Time) bool {
	if l.Client == nil || l.Capacity <= 0 || l.RefillRate < 0 {
		return true
	}
	ttl := l.TTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	prefix := strings.TrimSpace(l.KeyPrefix)
	if prefix == "" {
		prefix = "sip:rl"
	}
	tokensKey := fmt.Sprintf("%s:%s:tokens", prefix, key)
	tsKey := fmt.Sprintf("%s:%s:ts", prefix, key)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	res, err := tokenBucketLua.Run(ctx, l.Client, []string{tokensKey, tsKey},
		float64(now.UnixNano())/1e9,
		l.Capacity,
		l.RefillRate,
		int(ttl.Seconds()),
	).Int()
	if err != nil {
		// Fail open: rate limiting is best-effort.
		return true
	}
	return res == 1
}
