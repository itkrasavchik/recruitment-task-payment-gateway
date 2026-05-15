package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/repository/models"
)

type MerchantRepo struct {
	db *bun.DB
}

func NewMerchantRepo(db *bun.DB) *MerchantRepo {
	return &MerchantRepo{db: db}
}

func (r *MerchantRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Merchant, error) {
	m := new(models.Merchant)

	err := r.db.NewSelect().Model(m).
		Where("api_key = ?", apiKey).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrMerchantNotFound
		}
		return nil, fmt.Errorf("get merchant by api key: %w", err)
	}

	return modelToDomainMerchant(m), nil
}

func modelToDomainMerchant(m *models.Merchant) *domain.Merchant {
	return &domain.Merchant{
		ID:        m.ID,
		Name:      m.Name,
		APIKey:    m.APIKey,
		CreatedAt: m.CreatedAt,
	}
}
