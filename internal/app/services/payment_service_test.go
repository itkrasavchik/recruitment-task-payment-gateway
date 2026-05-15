package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/services"
	"github.com/evt/recruitment-task-payment-gateway/internal/pkg/bank"
)

// --- Mocks ---

type mockBankClient struct {
	resp bank.Response
	err  error
}

func (m *mockBankClient) ProcessPayment(_ context.Context, _ bank.Request) (bank.Response, error) {
	return m.resp, m.err
}

type mockPaymentRepo struct {
	createPendingFn func(ctx context.Context, p *domain.Payment) (*domain.Payment, bool, error)
	updateStatusFn  func(ctx context.Context, p *domain.Payment) error
	getFn           func(ctx context.Context, merchantID, paymentID string) (*domain.Payment, error)
}

func (m *mockPaymentRepo) CreatePending(ctx context.Context, p *domain.Payment) (*domain.Payment, bool, error) {
	return m.createPendingFn(ctx, p)
}

func (m *mockPaymentRepo) UpdateStatus(ctx context.Context, p *domain.Payment) error {
	return m.updateStatusFn(ctx, p)
}

func (m *mockPaymentRepo) GetByID(ctx context.Context, merchantID, paymentID string) (*domain.Payment, error) {
	return m.getFn(ctx, merchantID, paymentID)
}

type mockEventLogger struct {
	events []*domain.PaymentEvent
	err    error
}

func (m *mockEventLogger) Append(_ context.Context, event *domain.PaymentEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

type mockMerchantRepo struct {
	getFn func(ctx context.Context, apiKey string) (*domain.Merchant, error)
}

func (m *mockMerchantRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Merchant, error) {
	return m.getFn(ctx, apiKey)
}

// --- Helpers ---

func validIntent() domain.PaymentIntent {
	intent, _ := domain.NewPaymentIntent(
		"merchant-1", "idem-1", "4242424242424242",
		12, 2030, "123", 5000, "USD",
	)
	return intent
}

func passthruRepo() *mockPaymentRepo {
	return &mockPaymentRepo{
		createPendingFn: func(_ context.Context, p *domain.Payment) (*domain.Payment, bool, error) {
			p.ID = "pay-001"
			p.CreatedAt = time.Now()
			return p, false, nil
		},
		updateStatusFn: func(_ context.Context, _ *domain.Payment) error {
			return nil
		},
	}
}

// --- Payment service tests ---

func TestProcessPayment_Authorized(t *testing.T) {
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "txn-001"},
	}
	repo := passthruRepo()
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	payment, err := svc.ProcessPayment(context.Background(), validIntent())

	require.NoError(t, err)
	require.Equal(t, domain.StatusAuthorized, payment.Status)
	require.NotNil(t, payment.BankTransactionID)
	require.Equal(t, "txn-001", *payment.BankTransactionID)
	require.Nil(t, payment.DeclineReason)
	require.Equal(t, "4242", payment.CardLast4)
}

func TestProcessPayment_Declined(t *testing.T) {
	reason := "insufficient_funds"
	bankClient := &mockBankClient{
		resp: bank.Response{
			Authorized:    false,
			TransactionID: "txn-002",
			DeclineReason: reason,
		},
	}
	repo := passthruRepo()
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	payment, err := svc.ProcessPayment(context.Background(), validIntent())

	require.NoError(t, err)
	require.Equal(t, domain.StatusDeclined, payment.Status)
	require.NotNil(t, payment.DeclineReason)
	require.Equal(t, reason, *payment.DeclineReason)
	require.NotNil(t, payment.BankTransactionID)
	require.Equal(t, "txn-002", *payment.BankTransactionID)
}

func TestProcessPayment_BankError(t *testing.T) {
	bankClient := &mockBankClient{
		err: errors.New("connection refused"),
	}
	repo := passthruRepo()
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	payment, err := svc.ProcessPayment(context.Background(), validIntent())

	require.NoError(t, err)
	require.Equal(t, domain.StatusFailed, payment.Status)
	// Bank errors are now mapped to a generic reason, not the raw error.
	require.NotNil(t, payment.DeclineReason)
	require.Equal(t, "processing_error", *payment.DeclineReason)
	require.Nil(t, payment.BankTransactionID)
}

func TestProcessPayment_CreatePendingRepoError(t *testing.T) {
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "txn-003"},
	}
	repo := &mockPaymentRepo{
		createPendingFn: func(_ context.Context, _ *domain.Payment) (*domain.Payment, bool, error) {
			return nil, false, errors.New("db down")
		},
	}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	_, err := svc.ProcessPayment(context.Background(), validIntent())

	require.Error(t, err)
	require.Contains(t, err.Error(), "db down")
}

func TestProcessPayment_UpdateStatusRepoError(t *testing.T) {
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "txn-004"},
	}
	repo := &mockPaymentRepo{
		createPendingFn: func(_ context.Context, p *domain.Payment) (*domain.Payment, bool, error) {
			p.ID = "pay-001"
			return p, false, nil
		},
		updateStatusFn: func(_ context.Context, _ *domain.Payment) error {
			return errors.New("db write failed")
		},
	}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	_, err := svc.ProcessPayment(context.Background(), validIntent())

	require.Error(t, err)
	require.Contains(t, err.Error(), "db write failed")
}

func TestProcessPayment_IdempotentDuplicate(t *testing.T) {
	existing := &domain.Payment{
		ID:         "pay-existing",
		MerchantID: "merchant-1",
		Status:     domain.StatusAuthorized,
	}
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "should-not-be-called"},
	}
	repo := &mockPaymentRepo{
		createPendingFn: func(_ context.Context, _ *domain.Payment) (*domain.Payment, bool, error) {
			return existing, true, nil
		},
	}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	payment, err := svc.ProcessPayment(context.Background(), validIntent())

	require.NoError(t, err)
	require.Equal(t, "pay-existing", payment.ID)
	require.Equal(t, domain.StatusAuthorized, payment.Status)
}

func TestProcessPayment_InFlightDuplicate(t *testing.T) {
	existing := &domain.Payment{
		ID:         "pay-pending",
		MerchantID: "merchant-1",
		Status:     domain.StatusPending,
	}
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "should-not-be-called"},
	}
	repo := &mockPaymentRepo{
		createPendingFn: func(_ context.Context, _ *domain.Payment) (*domain.Payment, bool, error) {
			return existing, true, nil
		},
	}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, &mockEventLogger{})

	_, err := svc.ProcessPayment(context.Background(), validIntent())

	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrPaymentInFlight)
}

func TestGetPayment_Found(t *testing.T) {
	expected := &domain.Payment{
		ID:         "pay-001",
		MerchantID: "merchant-1",
		Status:     domain.StatusAuthorized,
	}
	repo := &mockPaymentRepo{
		getFn: func(_ context.Context, _, _ string) (*domain.Payment, error) {
			return expected, nil
		},
	}
	svc := services.NewPaymentService(repo, nil, 5*time.Second, &mockEventLogger{})

	payment, err := svc.GetPayment(context.Background(), "merchant-1", "pay-001")

	require.NoError(t, err)
	require.Equal(t, expected, payment)
}

func TestGetPayment_NotFound(t *testing.T) {
	repo := &mockPaymentRepo{
		getFn: func(_ context.Context, _, _ string) (*domain.Payment, error) {
			return nil, domain.ErrPaymentNotFound
		},
	}
	svc := services.NewPaymentService(repo, nil, 5*time.Second, &mockEventLogger{})

	_, err := svc.GetPayment(context.Background(), "merchant-1", "pay-999")

	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrPaymentNotFound)
}

// --- Audit event tests ---

func TestProcessPayment_AuditEvents_Authorized(t *testing.T) {
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "txn-100"},
	}
	repo := passthruRepo()
	events := &mockEventLogger{}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, events)

	_, err := svc.ProcessPayment(context.Background(), validIntent())
	require.NoError(t, err)

	require.Len(t, events.events, 2)
	require.Equal(t, domain.EventPaymentCreated, events.events[0].EventType)
	require.Equal(t, "pay-001", events.events[0].PaymentID)
	require.Equal(t, int64(5000), events.events[0].Data["amount"])
	require.Equal(t, "USD", events.events[0].Data["currency"])
	require.Equal(t, "4242", events.events[0].Data["card_last4"])
	require.Equal(t, "merchant-1", events.events[0].Data["merchant_id"])

	require.Equal(t, domain.EventBankAuthorized, events.events[1].EventType)
	require.Equal(t, "pay-001", events.events[1].PaymentID)
	require.Equal(t, "txn-100", events.events[1].Data["bank_transaction_id"])
}

func TestProcessPayment_AuditEvents_Declined(t *testing.T) {
	bankClient := &mockBankClient{
		resp: bank.Response{
			Authorized:    false,
			TransactionID: "txn-200",
			DeclineReason: "insufficient_funds",
		},
	}
	repo := passthruRepo()
	events := &mockEventLogger{}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, events)

	_, err := svc.ProcessPayment(context.Background(), validIntent())
	require.NoError(t, err)

	require.Len(t, events.events, 2)
	require.Equal(t, domain.EventPaymentCreated, events.events[0].EventType)
	require.Equal(t, domain.EventBankDeclined, events.events[1].EventType)
	require.Equal(t, "txn-200", events.events[1].Data["bank_transaction_id"])
	require.Equal(t, "insufficient_funds", events.events[1].Data["decline_reason"])
}

func TestProcessPayment_AuditEvents_BankFailed(t *testing.T) {
	bankClient := &mockBankClient{
		err: errors.New("timeout"),
	}
	repo := passthruRepo()
	events := &mockEventLogger{}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, events)

	_, err := svc.ProcessPayment(context.Background(), validIntent())
	require.NoError(t, err)

	require.Len(t, events.events, 2)
	require.Equal(t, domain.EventPaymentCreated, events.events[0].EventType)
	require.Equal(t, domain.EventBankFailed, events.events[1].EventType)
	require.Equal(t, "processing_error", events.events[1].Data["decline_reason"])
}

func TestProcessPayment_AuditEvents_IdempotentDuplicate_NoEvents(t *testing.T) {
	existing := &domain.Payment{
		ID:         "pay-existing",
		MerchantID: "merchant-1",
		Status:     domain.StatusAuthorized,
	}
	bankClient := &mockBankClient{}
	repo := &mockPaymentRepo{
		createPendingFn: func(_ context.Context, _ *domain.Payment) (*domain.Payment, bool, error) {
			return existing, true, nil
		},
	}
	events := &mockEventLogger{}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, events)

	_, err := svc.ProcessPayment(context.Background(), validIntent())
	require.NoError(t, err)
	require.Empty(t, events.events)
}

func TestProcessPayment_AuditLogFailure_DoesNotBreakPayment(t *testing.T) {
	bankClient := &mockBankClient{
		resp: bank.Response{Authorized: true, TransactionID: "txn-300"},
	}
	repo := passthruRepo()
	events := &mockEventLogger{err: errors.New("audit db down")}
	svc := services.NewPaymentService(repo, bankClient, 5*time.Second, events)

	payment, err := svc.ProcessPayment(context.Background(), validIntent())

	require.NoError(t, err)
	require.Equal(t, domain.StatusAuthorized, payment.Status)
	require.NotNil(t, payment.BankTransactionID)
	require.Equal(t, "txn-300", *payment.BankTransactionID)
}

// --- Merchant service tests ---

func TestAuthenticate_Valid(t *testing.T) {
	expected := &domain.Merchant{ID: "m-1", Name: "Test"}
	repo := &mockMerchantRepo{
		getFn: func(_ context.Context, _ string) (*domain.Merchant, error) {
			return expected, nil
		},
	}
	svc := services.NewMerchantService(repo)

	merchant, err := svc.Authenticate(context.Background(), "sk_test_key")

	require.NoError(t, err)
	require.Equal(t, expected, merchant)
}

func TestAuthenticate_Invalid(t *testing.T) {
	repo := &mockMerchantRepo{
		getFn: func(_ context.Context, _ string) (*domain.Merchant, error) {
			return nil, domain.ErrMerchantNotFound
		},
	}
	svc := services.NewMerchantService(repo)

	_, err := svc.Authenticate(context.Background(), "bad_key")

	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrMerchantNotFound)
}
