package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	router      *chi.Mux
	paymentSvc  PaymentService
	merchantSvc MerchantService
}

func NewServer(paymentSvc PaymentService, merchantSvc MerchantService) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		paymentSvc:  paymentSvc,
		merchantSvc: merchantSvc,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Use(recoveryMiddleware)
	s.router.Use(requestIDMiddleware)
	s.router.Use(loggingMiddleware)
	s.router.Use(metricsMiddleware)

	s.router.Handle("/metrics", promhttp.Handler())

	s.router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			slog.Error("failed to write health response", "error", err)
		}
	})

	s.router.Group(func(r chi.Router) {
		r.Use(merchantAuthMiddleware(s.merchantSvc))

		h := NewPaymentHandler(s.paymentSvc)
		r.Post("/payments", h.CreatePayment)
		r.Get("/payments/{paymentID}", h.GetPayment)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
