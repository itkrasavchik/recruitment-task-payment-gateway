package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/config"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/metrics"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/repository"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/repository/models"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/services"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/transport/httpserver"
	"github.com/evt/recruitment-task-payment-gateway/internal/pkg/bank"
)

const defaultShutdownTimeout = 10 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "error", err) //nolint:sloglint // global logger used in main
		os.Exit(1)
	}
}

func run() error {
	metrics.Register()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	bankTimeout, err := time.ParseDuration(cfg.BankTimeout)
	if err != nil {
		return fmt.Errorf("parse bank timeout: %w", err)
	}

	// Connect to DB via Bun
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.DatabaseURL)))
	db := bun.NewDB(sqldb, pgdialect.New())
	defer db.Close()

	ctx := context.Background()
	if pingErr := db.PingContext(ctx); pingErr != nil {
		return fmt.Errorf("ping db: %w", pingErr)
	}

	// Create tables
	if tableErr := createTables(ctx, db); tableErr != nil {
		return fmt.Errorf("create tables: %w", tableErr)
	}

	// Seed merchant
	if seedErr := seedMerchant(ctx, db); seedErr != nil {
		return fmt.Errorf("seed merchant: %w", seedErr)
	}
	slog.Info("database ready") //nolint:sloglint // global logger used in main

	// Repos
	merchantRepo := repository.NewMerchantRepo(db)
	paymentRepo := repository.NewPaymentRepo(db)

	// Services
	var bankClient bank.Client
	if cfg.BankMode == "http" {
		bankClient = bank.NewHTTPClient(cfg.BankURL, bankTimeout)
		slog.Info("using HTTP bank client", "url", cfg.BankURL) //nolint:sloglint // global logger used in main
	} else {
		bankClient = bank.NewSimulator()
		slog.Info("using in-process bank simulator") //nolint:sloglint // global logger used in main
	}
	merchantSvc := services.NewMerchantService(merchantRepo)
	eventRepo := repository.NewPaymentEventRepo(db)
	paymentSvc := services.NewPaymentService(paymentRepo, bankClient, bankTimeout, eventRepo)

	// HTTP server
	srv := httpserver.NewServer(paymentSvc, merchantSvc)
	httpSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           srv,
		ReadHeaderTimeout: defaultShutdownTimeout,
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "port", cfg.HTTPPort)
		errCh <- httpSrv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("received signal, shutting down", "signal", sig) //nolint:sloglint // global logger used in main
	case listenErr := <-errCh:
		if !errors.Is(listenErr, http.ErrServerClosed) {
			return listenErr
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func createTables(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewCreateTable().Model((*models.Merchant)(nil)).IfNotExists().Exec(ctx); err != nil {
		return fmt.Errorf("create merchants table: %w", err)
	}
	if _, err := db.NewCreateTable().Model((*models.Payment)(nil)).IfNotExists().Exec(ctx); err != nil {
		return fmt.Errorf("create payments table: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_merchant_idempotency "+
			"ON payments (merchant_id, idempotency_key)"); err != nil {
		return fmt.Errorf("create unique index: %w", err)
	}
	if _, err := db.NewCreateTable().Model((*models.PaymentEvent)(nil)).
		IfNotExists().
		ForeignKey(`("payment_id") REFERENCES "payments" ("id")`).
		Exec(ctx); err != nil {
		return fmt.Errorf("create payment_events table: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		"CREATE INDEX IF NOT EXISTS idx_payment_events_payment_id "+
			"ON payment_events (payment_id)"); err != nil {
		return fmt.Errorf("create payment_events index: %w", err)
	}
	return nil
}

func seedMerchant(ctx context.Context, db *bun.DB) error {
	m := &models.Merchant{
		ID:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name:   "Test Merchant",
		APIKey: "sk_test_merchant_key_001",
	}
	_, err := db.NewInsert().Model(m).On("CONFLICT (id) DO NOTHING").Exec(ctx)
	if err != nil {
		return fmt.Errorf("seed merchant: %w", err)
	}
	return nil
}
