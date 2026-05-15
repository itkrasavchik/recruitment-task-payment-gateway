package httpserver

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/common/server"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/metrics"
)

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", //nolint:gosec // logging request metadata is safe
					"request_id", getRequestID(r.Context()),
					"method", r.Method,
					"route", routePattern(r),
					"panic", rec,
				)
				server.InternalError("internal server error").Write(w)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		ctx := withRequestID(r.Context(), rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusWriter struct {
	http.ResponseWriter

	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		slog.Info("http request", //nolint:gosec // logging request metadata is safe
			"request_id", getRequestID(r.Context()),
			"method", r.Method,
			"route", routePattern(r),
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		route := routePattern(r)
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// routePattern returns the matched route template (e.g. "/payments/{paymentID}")
// instead of the actual URL path (e.g. "/payments/abc-123"). This keeps log
// cardinality low — logging raw paths creates unbounded unique values that
// blow up log indexing and aggregation costs in systems like Datadog or Loki.
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return r.URL.Path
	}
	pattern := rctx.RoutePattern()
	if pattern == "" {
		return r.URL.Path
	}
	return pattern
}
