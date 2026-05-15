package mocks

import (
	"context"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

type MockPaymentService struct {
	ProcessPaymentFn func(ctx context.Context, intent domain.PaymentIntent) (*domain.Payment, error)
	GetPaymentFn     func(ctx context.Context, merchantID, paymentID string) (*domain.Payment, error)
}

func (m *MockPaymentService) ProcessPayment(
	ctx context.Context,
	intent domain.PaymentIntent,
) (*domain.Payment, error) {
	return m.ProcessPaymentFn(ctx, intent)
}

func (m *MockPaymentService) GetPayment(
	ctx context.Context,
	merchantID, paymentID string,
) (*domain.Payment, error) {
	return m.GetPaymentFn(ctx, merchantID, paymentID)
}

type MockMerchantService struct {
	AuthenticateFn func(ctx context.Context, apiKey string) (*domain.Merchant, error)
}

func (m *MockMerchantService) Authenticate(
	ctx context.Context,
	apiKey string,
) (*domain.Merchant, error) {
	return m.AuthenticateFn(ctx, apiKey)
}
