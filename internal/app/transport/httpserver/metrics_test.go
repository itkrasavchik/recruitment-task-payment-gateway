package httpserver_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/metrics"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/transport/httpserver"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/transport/httpserver/mocks"
)

func TestMain(m *testing.M) {
	metrics.Register()
	os.Exit(m.Run())
}

func TestMetricsEndpoint(t *testing.T) {
	// CounterVec metrics don't appear in output until at least one label set
	// is initialised. Seed the ones that are only incremented inside the real
	// service (tests use mocks, so the real service code never runs).
	metrics.PaymentsTotal.WithLabelValues("authorized", "USD")
	metrics.PaymentAmountTotal.WithLabelValues("authorized", "USD")
	metrics.BankRequestsTotal.WithLabelValues("success")

	srv := httpserver.NewServer(&mocks.MockPaymentService{}, newTestMerchantSvc())

	// Make a request so the HTTP middleware metrics get seeded — the /metrics
	// request's own counters are recorded after promhttp writes the response,
	// so they won't appear in their own output.
	srv.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	for _, name := range []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"payments_total",
		"payment_amount_total",
		"bank_requests_total",
		"bank_request_duration_seconds",
		"bank_circuit_breaker_state",
	} {
		assert.Contains(t, body, name, "expected metric %q in /metrics output", name)
	}
}
