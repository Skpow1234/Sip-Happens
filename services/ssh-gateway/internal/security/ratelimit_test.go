package security

import (
	"testing"
	"time"
)

func TestSSHRateLimiter_allowByKey(t *testing.T) {
	rl := &SSHRateLimiter{Window: time.Minute, Limit: 2, hits: map[string][]time.Time{}}
	now := time.Now()
	if !rl.allow("fp:abc", now) || !rl.allow("fp:abc", now) {
		t.Fatal("expected first two attempts")
	}
	if rl.allow("fp:abc", now) {
		t.Fatal("expected third attempt blocked")
	}
	if !rl.allow("ip:1.2.3.4", now) {
		t.Fatal("expected separate IP bucket")
	}
}
