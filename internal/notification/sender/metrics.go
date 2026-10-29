package sender

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/furknozg/coldbru/internal/notification/provider"
)

// Channels and the final delivery statuses the worker produces, for series that exist from the start.
var (
	channels      = []string{"sms", "email"}
	finalStatuses = []string{"sent", "failed", "skipped_no_consent", "cancelled"}
	codeClasses   = []string{"2xx", "4xx", "5xx", "other", "error"}
)

// metrics are the worker's Prometheus series (D-079). Labels are the channel and a small closed
// set; there is no campaign, customer or delivery label, so cardinality stays fixed.
type metrics struct {
	deliveries *prometheus.CounterVec
	requests   *prometheus.CounterVec
	rateLimits *prometheus.CounterVec
	latency    *prometheus.HistogramVec
}

// newMetrics registers the series on reg. Every channel's series start at zero, so a dashboard
// shows them before the first message.
func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		deliveries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "deliveries_total",
			Help: "Deliveries that reached a final status through the send worker, by channel and status.",
		}, []string{"channel", "status"}),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "provider_requests_total",
			Help: "Requests to the provider by channel and response class (2xx, 4xx, 5xx, other, error for no response).",
		}, []string{"channel", "code_class"}),
		rateLimits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "provider_429_total",
			Help: "Provider answers with status 429 by channel.",
		}, []string{"channel"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "send_latency_seconds",
			Help:    "Duration of provider requests in seconds by channel.",
			Buckets: prometheus.DefBuckets,
		}, []string{"channel"}),
	}
	reg.MustRegister(m.deliveries, m.requests, m.rateLimits, m.latency)
	for _, ch := range channels {
		for _, s := range finalStatuses {
			m.deliveries.WithLabelValues(ch, s)
		}
		for _, c := range codeClasses {
			m.requests.WithLabelValues(ch, c)
		}
		m.rateLimits.WithLabelValues(ch)
		m.latency.WithLabelValues(ch)
	}
	return m
}

// providerCall records one provider request: its response class, a 429, and its duration.
func (m *metrics) providerCall(channel string, status int, seconds float64) {
	m.requests.WithLabelValues(channel, provider.CodeClass(status)).Inc()
	if status == 429 {
		m.rateLimits.WithLabelValues(channel).Inc()
	}
	m.latency.WithLabelValues(channel).Observe(seconds)
}

func (m *metrics) final(channel, status string) { m.deliveries.WithLabelValues(channel, status).Inc() }
