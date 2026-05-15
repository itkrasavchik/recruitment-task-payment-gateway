package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/metrics"
	"github.com/evt/recruitment-task-payment-gateway/internal/pkg/bank"
)

const genericBankFailureReason = "processing_error"

type PaymentService struct {
	repo        PaymentRepository
	bankClient  bank.Client
	bankTimeout time.Duration
	log         *slog.Logger
	events      EventLogger
}

func NewPaymentService(
	repo PaymentRepository,
	bankClient bank.Client,
	bankTimeout time.Duration,
	events EventLogger,
) *PaymentService {
	return &PaymentService{
		repo:        repo,
		bankClient:  bankClient,
		bankTimeout: bankTimeout,
		log:         slog.Default(),
		events:      events,
	}
}

func (s *PaymentService) ProcessPayment(ctx context.Context, intent domain.PaymentIntent) (*domain.Payment, error) {
	// Step 1: Insert a pending record to atomically claim the idempotency key.
	// If the key already exists, return the existing payment (idempotent).
	pending := &domain.Payment{
		MerchantID:     intent.MerchantID,
		IdempotencyKey: intent.IdempotencyKey,
		CardLast4:      intent.CardLast4(),
		ExpiryMonth:    intent.ExpiryMonth,
		ExpiryYear:     intent.ExpiryYear,
		Amount:         intent.Amount,
		Currency:       intent.Currency,
		Status:         domain.StatusPending,
	}

	payment, existed, err := s.repo.CreatePending(ctx, pending)
	if err != nil {
		return nil, fmt.Errorf("create pending payment: %w", err)
	}
	if existed {
		// If the existing payment is still pending, another request is in-flight.
		if payment.IsPending() {
			return nil, domain.ErrPaymentInFlight
		}
		return payment, nil
	}

	s.logEvent(ctx, domain.NewPaymentEvent(payment.ID, domain.EventPaymentCreated, map[string]any{
		"amount":      intent.Amount,
		"currency":    intent.Currency,
		"card_last4":  intent.CardLast4(),
		"merchant_id": intent.MerchantID,
	}))

	// Step 2: Call the bank. The idempotency key is now claimed — no other
	// concurrent request can proceed past Step 1 for this key.
	s.log.InfoContext(ctx, "processing bank authorization",
		"merchant_id", intent.MerchantID,
		"card_last4", intent.CardLast4(),
		"amount", intent.Amount,
		"currency", intent.Currency,
	)

	bankCtx, cancel := context.WithTimeout(ctx, s.bankTimeout)
	defer cancel()

	bankResp, bankErr := s.bankClient.ProcessPayment(bankCtx, bank.Request{
		CardNumber:  intent.CardNumber,
		ExpiryMonth: intent.ExpiryMonth,
		ExpiryYear:  intent.ExpiryYear,
		CVV:         intent.CVV,
		Amount:      intent.Amount,
		Currency:    intent.Currency,
	})

	// Step 3: Update the pending record with the bank result.
	switch {
	case bankErr != nil:
		s.log.ErrorContext(ctx, "bank call failed", "error", bankErr)
		payment.MarkFailed(genericBankFailureReason)
		s.logEvent(ctx, domain.NewPaymentEvent(payment.ID, domain.EventBankFailed, map[string]any{
			"decline_reason": genericBankFailureReason,
		}))
	case !bankResp.Authorized:
		payment.MarkDeclined(bankResp.DeclineReason, bankResp.TransactionID)
		s.logEvent(ctx, domain.NewPaymentEvent(payment.ID, domain.EventBankDeclined, map[string]any{
			"bank_transaction_id": bankResp.TransactionID,
			"decline_reason":      bankResp.DeclineReason,
		}))
	default:
		payment.MarkAuthorized(bankResp.TransactionID)
		s.logEvent(ctx, domain.NewPaymentEvent(payment.ID, domain.EventBankAuthorized, map[string]any{
			"bank_transaction_id": bankResp.TransactionID,
		}))
	}

	status := payment.Status.String()
	currency := payment.Currency
	metrics.PaymentsTotal.WithLabelValues(status, currency).Inc()
	metrics.PaymentAmountTotal.WithLabelValues(status, currency).Add(float64(payment.Amount))

	if updateErr := s.repo.UpdateStatus(ctx, payment); updateErr != nil {
		// Log the bank result so it can be reconciled even if the DB write fails.
		s.log.ErrorContext(ctx, "failed to persist bank result",
			"payment_id", payment.ID,
			"status", payment.Status.String(),
			"bank_transaction_id", payment.BankTransactionID,
			"error", updateErr,
		)
		return nil, fmt.Errorf("update payment status: %w", updateErr)
	}

	return payment, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, merchantID, paymentID string) (*domain.Payment, error) {
	return s.repo.GetByID(ctx, merchantID, paymentID)
}

func (s *PaymentService) logEvent(ctx context.Context, event *domain.PaymentEvent) {
	if err := s.events.Append(ctx, event); err != nil {
		s.log.ErrorContext(ctx, "failed to append audit event",
			"payment_id", event.PaymentID,
			"event_type", event.EventType,
			"error", err,
		)
	}
}
