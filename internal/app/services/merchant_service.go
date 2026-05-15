package services

import (
	"context"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

type MerchantService struct {
	repo MerchantRepository
}

func NewMerchantService(repo MerchantRepository) *MerchantService {
	return &MerchantService{repo: repo}
}

func (s *MerchantService) Authenticate(ctx context.Context, apiKey string) (*domain.Merchant, error) {
	return s.repo.GetByAPIKey(ctx, apiKey)
}
