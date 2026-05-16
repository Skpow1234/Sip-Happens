package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"sip-happens/pkg/config"
	"sip-happens/services/core-api/internal/testutil"
)

func TestReadyz_NATSRequiredWithoutURL(t *testing.T) {
	ctx := context.Background()
	tdb, err := testutil.OpenMigrated(ctx)
	if err != nil {
		t.Skipf("db not available: %v", err)
	}
	defer func() { _ = tdb.CloseFn() }()

	srv := &Server{
		DB:              tdb.DB,
		OutboxPublisher: config.OutboxPublisherNATS,
		NATSURL:         "",
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
