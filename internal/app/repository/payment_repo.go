package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/repository/models"
)

type PaymentRepo struct {
	db *bun.DB
}

func NewPaymentRepo(db *bun.DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

// CreatePending inserts a payment with "pending" status to claim the idempotency key.
// Returns (existing_payment, true, nil) if the key was already claimed.
// Returns (new_payment, false, nil) on successful insert.
func (r *PaymentRepo) CreatePending(
	ctx context.Context,
	p *domain.Payment,
) (*domain.Payment, bool, error) {
	m := domainToModelPayment(p)

	_, err := r.db.NewInsert().Model(m).Returning("*").Exec(ctx)
	if err != nil {
		if isUniqueViolation(err) {
			existing, getErr := r.getByIdempotencyKey(ctx, p.MerchantID, p.IdempotencyKey)
			if getErr != nil {
				return nil, false, getErr
			}
			return existing, true, nil
		}
		return nil, false, fmt.Errorf("create payment: %w", err)
	}

	return modelToDomainPayment(m), false, nil
}

// UpdateStatus transitions a payment from "pending" to a terminal status.
// The WHERE status='pending' guard prevents overwriting a finalized payment.
func (r *PaymentRepo) UpdateStatus(ctx context.Context, p *domain.Payment) error {
	res, err := r.db.NewUpdate().
		Model((*models.Payment)(nil)).
		Set("status = ?", p.Status.String()).
		Set("decline_reason = ?", p.DeclineReason).
		Set("bank_transaction_id = ?", p.BankTransactionID).
		Set("updated_at = current_timestamp").
		Where("id = ?", p.ID).
		Where("status = ?", domain.StatusPending.String()).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrPaymentAlreadyFinalized
	}
	return nil
}

func (r *PaymentRepo) GetByID(
	ctx context.Context,
	merchantID, paymentID string,
) (*domain.Payment, error) {
	m := new(models.Payment)

	err := r.db.NewSelect().Model(m).
		Where("id = ?", paymentID).
		Where("merchant_id = ?", merchantID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("get payment by id: %w", err)
	}

	return modelToDomainPayment(m), nil
}

func (r *PaymentRepo) getByIdempotencyKey(
	ctx context.Context,
	merchantID, idempotencyKey string,
) (*domain.Payment, error) {
	m := new(models.Payment)

	err := r.db.NewSelect().Model(m).
		Where("merchant_id = ?", merchantID).
		Where("idempotency_key = ?", idempotencyKey).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("get payment by idempotency key: %w", err)
	}

	return modelToDomainPayment(m), nil
}

func isUniqueViolation(err error) bool {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation()
	}
	return false
}

func modelToDomainPayment(m *models.Payment) *domain.Payment {
	return &domain.Payment{
		ID:                m.ID,
		MerchantID:        m.MerchantID,
		IdempotencyKey:    m.IdempotencyKey,
		CardLast4:         m.CardLast4,
		ExpiryMonth:       m.ExpiryMonth,
		ExpiryYear:        m.ExpiryYear,
		Amount:            m.Amount,
		Currency:          m.Currency,
		Status:            domain.PaymentStatus(m.Status),
		DeclineReason:     m.DeclineReason,
		BankTransactionID: m.BankTransactionID,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

func domainToModelPayment(p *domain.Payment) *models.Payment {
	return &models.Payment{
		MerchantID:        p.MerchantID,
		IdempotencyKey:    p.IdempotencyKey,
		CardLast4:         p.CardLast4,
		ExpiryMonth:       p.ExpiryMonth,
		ExpiryYear:        p.ExpiryYear,
		Amount:            p.Amount,
		Currency:          p.Currency,
		Status:            p.Status.String(),
		DeclineReason:     p.DeclineReason,
		BankTransactionID: p.BankTransactionID,
	}
}
