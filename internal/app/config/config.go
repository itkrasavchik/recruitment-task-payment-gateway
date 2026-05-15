package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	HTTPPort    string
	BankTimeout string
	BankMode    string // "simulator" or "http"
	BankURL     string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/payments?sslmode=disable"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		BankTimeout: getEnv("BANK_TIMEOUT", "10s"),
		BankMode:    getEnv("BANK_MODE", "http"),
		BankURL:     getEnv("BANK_URL", "http://localhost:9090"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
