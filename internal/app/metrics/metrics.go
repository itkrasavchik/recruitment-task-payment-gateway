package metrics //nolint:revive // "metrics" mirrors the Prometheus domain; runtime/metrics is rarely imported directly

import "github.com/prometheus/client_golang/prometheus"

// HTTP middleware metrics.
var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "route", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

// Payment outcome metrics.
var (
	PaymentsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total number of processed payments.",
		},
		[]string{"status", "currency"},
	)
	PaymentAmountTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_amount_total",
			Help: "Total payment amount in minor units.",
		},
		[]string{"status", "currency"},
	)
)

// Bank client metrics.
var (
	BankRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bank_requests_total",
			Help: "Total number of bank requests.",
		},
		[]string{"result"},
	)
	BankRequestDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "bank_request_duration_seconds",
			Help:    "Duration of bank requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)
	BankCircuitBreakerState = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "bank_circuit_breaker_state",
			Help: "Current state of the bank circuit breaker (0=closed, 1=half-open, 2=open).",
		},
	)
)

// Register registers all metrics with the default Prometheus registry.
func Register() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		PaymentsTotal,
		PaymentAmountTotal,
		BankRequestsTotal,
		BankRequestDuration,
		BankCircuitBreakerState,
	)
}
