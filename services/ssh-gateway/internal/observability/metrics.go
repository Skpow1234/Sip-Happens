package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	connectionsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ssh_gateway_connections_total",
		Help: "SSH sessions that passed rate limiting and started the TUI.",
	})
	connectionErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ssh_gateway_connection_errors_total",
		Help: "SSH session rejections (rate limit, etc.).",
	}, []string{"reason"})
)

func init() {
	prometheus.MustRegister(connectionsTotal, connectionErrorsTotal)
}

// RecordConnection increments successful session starts.
func RecordConnection() {
	connectionsTotal.Inc()
}

// RecordRateLimited increments rate-limited connection attempts.
func RecordRateLimited() {
	connectionErrorsTotal.WithLabelValues("rate_limited").Inc()
}

// MetricsHandler serves Prometheus metrics for ssh-gateway.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
