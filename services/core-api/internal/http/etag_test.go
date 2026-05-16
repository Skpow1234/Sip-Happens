package http

import (
	"testing"
	"time"
)

func TestProductWeakETagRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Nanosecond)
	tag := productWeakETag(now)
	if !parseProductIfMatch(tag, now) {
		t.Fatalf("etag %q should match %v", tag, now)
	}
	if parseProductIfMatch(tag, now.Add(time.Second)) {
		t.Fatal("should not match different time")
	}
}

func TestParseProductIfMatch_EmptyAllows(t *testing.T) {
	if !parseProductIfMatch("", time.Now()) {
		t.Fatal("empty If-Match should allow update")
	}
}
