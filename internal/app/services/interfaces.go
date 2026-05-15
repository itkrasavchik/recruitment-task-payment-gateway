package services

import (
	"context"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

type PaymentRepository interface {
	CreatePending(ctx context.Context, p *domain.Payment) (*domain.Payment, bool, error)
	UpdateStatus(ctx context.Context, p *domain.Payment) error
	GetByID(ctx context.Context, merchantID, paymentID string) (*domain.Payment, error)
}

type MerchantRepository interface {
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.Merchant, error)
}

type EventLogger interface {
	Append(ctx context.Context, event *domain.PaymentEvent) error
}
