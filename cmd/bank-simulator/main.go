package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/evt/recruitment-task-payment-gateway/internal/pkg/bank"
)

const defaultReadHeaderTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "9090"
	}

	sim := bank.NewSimulator()
	handler := bank.NewHandler(sim)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}

	//nolint:sloglint,gosec // global logger in main; port from env
	slog.Info("starting bank simulator", "port", port)

	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err) //nolint:sloglint // global logger used in main
		os.Exit(1)
	}
}
