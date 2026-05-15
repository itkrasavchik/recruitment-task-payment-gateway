package repository

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/repository/models"
)

type PaymentEventRepo struct {
	db *bun.DB
}

func NewPaymentEventRepo(db *bun.DB) *PaymentEventRepo {
	return &PaymentEventRepo{db: db}
}

func (r *PaymentEventRepo) Append(ctx context.Context, event *domain.PaymentEvent) error {
	m := &models.PaymentEvent{
		ID:        event.ID,
		PaymentID: event.PaymentID,
		EventType: string(event.EventType),
		Data:      event.Data,
		CreatedAt: event.CreatedAt,
	}

	if _, err := r.db.NewInsert().Model(m).Exec(ctx); err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}
	return nil
}
