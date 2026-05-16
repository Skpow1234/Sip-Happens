package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDBStatsCollectorDescribeCollect(t *testing.T) {
	c := NewDBStatsCollector(nil)
	ch := make(chan *prometheus.Desc, 4)
	c.Describe(ch)
	if len(ch) != 4 {
		t.Fatalf("expected 4 descriptors, got %d", len(ch))
	}
	// Collect with nil db should not panic.
	c.Collect(make(chan prometheus.Metric, 4))
}
