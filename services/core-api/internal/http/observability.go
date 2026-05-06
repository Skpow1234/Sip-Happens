package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const HeaderRequestID = "X-Request-Id"

type ctxKey string

const requestIDKey ctxKey = "request_id"

func RequestIDFromHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(HeaderRequestID))
}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := RequestIDFromHeader(r)
		if rid == "" {
			rid = newRequestID()
			r.Header.Set(HeaderRequestID, rid)
		}
		w.Header().Set(HeaderRequestID, rid)
		next.ServeHTTP(w, r)
	})
}

type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "sip_happens_http_requests_total",
				Help: "Total HTTP requests.",
			},
			[]string{"method", "path", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sip_happens_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
	}
	prometheus.MustRegister(m.RequestsTotal, m.RequestDuration)
	return m
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func AccessLog(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}

			next.ServeHTTP(sw, r)

			path := r.URL.Path
			// Reduce label cardinality a bit (no query string).
			method := r.Method
			status := sw.status
			dur := time.Since(start).Seconds()

			if m != nil {
				ls := prometheus.Labels{
					"method": method,
					"path":   path,
					"status": itoa(status),
				}
				m.RequestsTotal.With(ls).Inc()
				m.RequestDuration.With(ls).Observe(dur)
			}

			remoteIP := clientIP(r)
			entry := map[string]any{
				"ts":          time.Now().UTC().Format(time.RFC3339Nano),
				"level":       "info",
				"msg":         "http_request",
				"request_id":  RequestIDFromHeader(r),
				"method":      method,
				"path":        path,
				"status":      status,
				"duration_ms": int64(time.Since(start) / time.Millisecond),
				"remote_ip":   remoteIP,
			}
			b, _ := json.Marshal(entry)
			log.Println(string(b))
		})
	}
}

func clientIP(r *http.Request) string {
	// Trusted proxy handling can be added later; for now keep it simple.
	return r.RemoteAddr
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func itoa(n int) string {
	// fast enough for small ints
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(buf[i:])
}

