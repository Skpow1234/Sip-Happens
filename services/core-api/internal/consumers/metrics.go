package consumers

import "github.com/prometheus/client_golang/prometheus"

var (
	DLQPublishedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sip_happens_nats_dlq_published_total",
			Help: "JetStream messages published to the DLQ subject after handler failure",
		},
		[]string{"stream", "consumer"},
	)
	HandlerFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sip_happens_nats_handler_failures_total",
			Help: "JetStream consumer handler failures before DLQ publish",
		},
		[]string{"stream", "consumer", "event_type"},
	)
)

func init() {
	prometheus.MustRegister(DLQPublishedTotal, HandlerFailuresTotal)
}
