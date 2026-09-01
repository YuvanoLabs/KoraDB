package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics is the bounded, process-local telemetry collection for KoraDB's
// unary gRPC service. It intentionally never uses request bodies, query
// values, API keys, document IDs, or principal names as labels.
type Metrics struct {
	inFlight atomic.Int64

	mu       sync.Mutex
	outcomes map[metricKey]metricValue
}

type metricKey struct {
	method string
	code   string
}

type metricValue struct {
	count         uint64
	durationNanos uint64
}

// NewMetrics returns an empty service-metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{outcomes: make(map[metricKey]metricValue)}
}

func (m *Metrics) begin() {
	if m != nil {
		m.inFlight.Add(1)
	}
}

func (m *Metrics) record(method, code string, duration time.Duration) {
	if m == nil {
		return
	}
	m.inFlight.Add(-1)
	m.mu.Lock()
	key := metricKey{method: method, code: code}
	value := m.outcomes[key]
	value.count++
	if duration > 0 {
		value.durationNanos += uint64(duration)
	}
	m.outcomes[key] = value
	m.mu.Unlock()
}

// HTTPHandler exposes metrics in Prometheus's text exposition format. The
// server command only permits this endpoint on loopback; callers should use a
// local collector or authenticated reverse proxy rather than publishing it.
func (m *Metrics) HTTPHandler() http.Handler {
	return http.HandlerFunc(m.serveHTTP)
}

func (m *Metrics) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != "/metrics" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	m.mu.Lock()
	keys := make([]metricKey, 0, len(m.outcomes))
	values := make(map[metricKey]metricValue, len(m.outcomes))
	for key, value := range m.outcomes {
		keys = append(keys, key)
		values[key] = value
	}
	m.mu.Unlock()
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method == keys[j].method {
			return keys[i].code < keys[j].code
		}
		return keys[i].method < keys[j].method
	})

	_, _ = fmt.Fprintln(w, "# HELP koradb_requests_in_flight Current unary gRPC requests being handled.")
	_, _ = fmt.Fprintln(w, "# TYPE koradb_requests_in_flight gauge")
	_, _ = fmt.Fprintf(w, "koradb_requests_in_flight %d\n", m.inFlight.Load())
	_, _ = fmt.Fprintln(w, "# HELP koradb_requests_total Completed unary gRPC requests by method and gRPC code.")
	_, _ = fmt.Fprintln(w, "# TYPE koradb_requests_total counter")
	_, _ = fmt.Fprintln(w, "# HELP koradb_request_duration_seconds Completed unary gRPC request duration by method and gRPC code.")
	_, _ = fmt.Fprintln(w, "# TYPE koradb_request_duration_seconds summary")
	for _, key := range keys {
		value := values[key]
		labels := fmt.Sprintf("method=\"%s\",code=\"%s\"", prometheusLabel(key.method), prometheusLabel(key.code))
		_, _ = fmt.Fprintf(w, "koradb_requests_total{%s} %d\n", labels, value.count)
		_, _ = fmt.Fprintf(w, "koradb_request_duration_seconds_sum{%s} %.9f\n", labels, float64(value.durationNanos)/float64(time.Second))
		_, _ = fmt.Fprintf(w, "koradb_request_duration_seconds_count{%s} %d\n", labels, value.count)
	}
}

func prometheusLabel(value string) string {
	return strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\"", "\\\"").Replace(value)
}
