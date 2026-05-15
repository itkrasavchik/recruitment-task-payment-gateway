package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/common/server"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/transport/httpserver"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/transport/httpserver/mocks"
)

const testAPIKey = "sk_test_key"
const testMerchantID = "merchant-001"

func newTestMerchantSvc() *mocks.MockMerchantService {
	return &mocks.MockMerchantService{
		AuthenticateFn: func(_ context.Context, apiKey string) (*domain.Merchant, error) {
			if apiKey == testAPIKey {
				return &domain.Merchant{ID: testMerchantID, Name: "Test"}, nil
			}
			return nil, domain.ErrMerchantNotFound
		},
	}
}

func makePaymentBody(t *testing.T, cardNumber, cvv, currency string, amount int64, expiryMonth, expiryYear int) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"card_number":  cardNumber,
		"cvv":          cvv,
		"currency":     currency,
		"amount":       amount,
		"expiry_month": expiryMonth,
		"expiry_year":  expiryYear,
	})
	require.NoError(t, err)
	return body
}

func TestCreatePayment_Authorized(t *testing.T) {
	payment := &domain.Payment{
		ID:          "pay-001",
		MerchantID:  testMerchantID,
		CardLast4:   "4242",
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		Amount:      5000,
		Currency:    "USD",
		Status:      domain.StatusAuthorized,
		CreatedAt:   time.Now(),
	}
	txnID := "bank-txn-001"
	payment.BankTransactionID = &txnID

	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			return payment, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4242424242424242", "123", "USD", 5000, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Idempotency-Key", "idem-001")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp httpserver.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "authorized", resp.Status)
	assert.Equal(t, "************4242", resp.CardLast4)
	assert.Equal(t, int64(5000), resp.Amount)
}

func TestCreatePayment_Declined(t *testing.T) {
	reason := "insufficient_funds"
	payment := &domain.Payment{
		ID:            "pay-002",
		MerchantID:    testMerchantID,
		CardLast4:     "0010",
		ExpiryMonth:   12,
		ExpiryYear:    2030,
		Amount:        5000,
		Currency:      "USD",
		Status:        domain.StatusDeclined,
		DeclineReason: &reason,
		CreatedAt:     time.Now(),
	}

	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			return payment, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4000000000000010", "123", "USD", 5000, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Idempotency-Key", "idem-002")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp httpserver.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "declined", resp.Status)
	assert.Equal(t, "insufficient_funds", resp.DeclineReason)
}

func TestGetPayment(t *testing.T) {
	payment := &domain.Payment{
		ID:          "pay-001",
		MerchantID:  testMerchantID,
		CardLast4:   "4242",
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		Amount:      5000,
		Currency:    "USD",
		Status:      domain.StatusAuthorized,
		CreatedAt:   time.Now(),
	}

	paymentSvc := &mocks.MockPaymentService{
		GetPaymentFn: func(_ context.Context, merchantID, paymentID string) (*domain.Payment, error) {
			assert.Equal(t, testMerchantID, merchantID)
			assert.Equal(t, "pay-001", paymentID)
			return payment, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	req := httptest.NewRequest(http.MethodGet, "/payments/pay-001", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp httpserver.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "pay-001", resp.ID)
	assert.Equal(t, "************4242", resp.CardLast4)
}

func TestGetPayment_NotFound(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{
		GetPaymentFn: func(_ context.Context, _, _ string) (*domain.Payment, error) {
			return nil, domain.ErrPaymentNotFound
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	req := httptest.NewRequest(http.MethodGet, "/payments/pay-999", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreatePayment_Idempotency(t *testing.T) {
	callCount := 0
	payment := &domain.Payment{
		ID:          "pay-001",
		MerchantID:  testMerchantID,
		CardLast4:   "4242",
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		Amount:      5000,
		Currency:    "USD",
		Status:      domain.StatusAuthorized,
		CreatedAt:   time.Now(),
	}

	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, intent domain.PaymentIntent) (*domain.Payment, error) {
			callCount++
			assert.Equal(t, "idem-same", intent.IdempotencyKey)
			return payment, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	for range 3 {
		body := makePaymentBody(t, "4242424242424242", "123", "USD", 5000, 12, 2030)
		req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
		req.Header.Set("Idempotency-Key", "idem-same")

		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)

		var resp httpserver.PaymentResponse
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.Equal(t, "pay-001", resp.ID)
	}

	assert.Equal(t, 3, callCount)
}

func TestCreatePayment_ValidationErrors(t *testing.T) {
	// ProcessPaymentFn should never be called for invalid input —
	// the handler rejects it before reaching the service.
	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			t.Fatal("service should not be called for invalid input")
			return &domain.Payment{}, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	tests := []struct {
		name       string
		cardNumber string
		cvv        string
		currency   string
		amount     int64
		month      int
		year       int
		wantMsg    string
	}{
		{
			name:       "bad luhn",
			cardNumber: "4242424242424241",
			cvv:        "123",
			currency:   "USD",
			amount:     100,
			month:      12,
			year:       2030,
			wantMsg:    "Luhn",
		},
		{
			name:       "expired card",
			cardNumber: "4242424242424242",
			cvv:        "123",
			currency:   "USD",
			amount:     100,
			month:      1,
			year:       2020,
			wantMsg:    "expired",
		},
		{
			name:       "invalid amount",
			cardNumber: "4242424242424242",
			cvv:        "123",
			currency:   "USD",
			amount:     0,
			month:      12,
			year:       2030,
			wantMsg:    "amount",
		},
		{
			name:       "invalid currency",
			cardNumber: "4242424242424242",
			cvv:        "123",
			currency:   "XYZ",
			amount:     100,
			month:      12,
			year:       2030,
			wantMsg:    "currency",
		},
		{
			name:       "bad cvv",
			cardNumber: "4242424242424242",
			cvv:        "12",
			currency:   "USD",
			amount:     100,
			month:      12,
			year:       2030,
			wantMsg:    "CVV",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := makePaymentBody(t, tt.cardNumber, tt.cvv, tt.currency, tt.amount, tt.month, tt.year)
			req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+testAPIKey)
			req.Header.Set("Idempotency-Key", "idem-val-"+tt.name)

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)

			var resp server.HTTPError
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			require.Equal(t, server.CodeValidationError, resp.Code)
			require.Contains(t, resp.Message, tt.wantMsg)
		})
	}
}

func TestCreatePayment_MissingAuth(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4242424242424242", "123", "USD", 100, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Idempotency-Key", "idem-noauth")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreatePayment_InvalidAPIKey(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4242424242424242", "123", "USD", 100, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer bad_key")
	req.Header.Set("Idempotency-Key", "idem-badkey")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreatePayment_MissingIdempotencyKey(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4242424242424242", "123", "USD", 100, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePayment_BankTimeout(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			reason := "context deadline exceeded"
			return &domain.Payment{
				ID:            "pay-timeout",
				MerchantID:    testMerchantID,
				CardLast4:     "4242",
				ExpiryMonth:   12,
				ExpiryYear:    2030,
				Amount:        5000,
				Currency:      "USD",
				Status:        domain.StatusFailed,
				DeclineReason: &reason,
				CreatedAt:     time.Now(),
			}, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4242424242424242", "123", "USD", 5000, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Idempotency-Key", "idem-timeout")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp httpserver.PaymentResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "failed", resp.Status)
}

func TestCreatePayment_InFlight(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			return nil, domain.ErrPaymentInFlight
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	body := makePaymentBody(t, "4242424242424242", "123", "USD", 5000, 12, 2030)
	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Idempotency-Key", "idem-inflight")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)

	var resp server.HTTPError
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, server.CodeConflict, resp.Code)
	assert.Equal(t, "payment is being processed", resp.Message)
}

func TestCreatePayment_ConcurrentIdempotent(t *testing.T) {
	payment := &domain.Payment{
		ID:          "pay-concurrent",
		MerchantID:  testMerchantID,
		CardLast4:   "4242",
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		Amount:      5000,
		Currency:    "USD",
		Status:      domain.StatusAuthorized,
		CreatedAt:   time.Now(),
	}

	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			return payment, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	var wg sync.WaitGroup
	results := make([]int, 10)
	body := makePaymentBody(t, "4242424242424242", "123", "USD", 5000, 12, 2030)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+testAPIKey)
			req.Header.Set("Idempotency-Key", "idem-concurrent")

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			results[idx] = w.Code
		}(i)
	}

	wg.Wait()

	for _, code := range results {
		require.Equal(t, http.StatusCreated, code)
	}
}

func TestGetPayment_CrossMerchantIsolation(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{
		GetPaymentFn: func(_ context.Context, merchantID, paymentID string) (*domain.Payment, error) {
			// Repo scopes by merchant — merchant B's ID won't match merchant A's payment.
			assert.Equal(t, testMerchantID, merchantID)
			assert.Equal(t, "pay-other", paymentID)
			return nil, domain.ErrPaymentNotFound
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	req := httptest.NewRequest(http.MethodGet, "/payments/pay-other", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreatePayment_AuthHeaderEdgeCases(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	tests := []struct {
		name   string
		header string
		status int
	}{
		{name: "no Bearer prefix", header: "sk_test_merchant_key_001", status: http.StatusUnauthorized},
		{name: "Bearer with no key", header: "Bearer ", status: http.StatusUnauthorized},
		{name: "lowercase bearer", header: "bearer sk_test_merchant_key_001", status: http.StatusUnauthorized},
		{name: "Basic instead of Bearer", header: "Basic sk_test_merchant_key_001", status: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := makePaymentBody(t, "4242424242424242", "123", "USD", 100, 12, 2030)
			req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
			req.Header.Set("Authorization", tt.header)
			req.Header.Set("Idempotency-Key", "idem-auth-"+tt.name)

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func TestMethodNotAllowed(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/payments"},
		{http.MethodDelete, "/payments"},
		{http.MethodPatch, "/payments/pay-001"},
		{http.MethodDelete, "/payments/pay-001"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer "+testAPIKey)

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestCreatePayment_OversizedBody(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, _ domain.PaymentIntent) (*domain.Payment, error) {
			t.Fatal("service should not be called for oversized body")
			return &domain.Payment{}, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	// 2 MB body — exceeds the 1 MB MaxBytesReader limit.
	oversized := strings.Repeat("x", 2<<20)
	req := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(oversized))
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Idempotency-Key", "idem-oversized")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePayment_AmountBoundaries(t *testing.T) {
	paymentSvc := &mocks.MockPaymentService{
		ProcessPaymentFn: func(_ context.Context, intent domain.PaymentIntent) (*domain.Payment, error) {
			return &domain.Payment{
				ID:          "pay-boundary",
				MerchantID:  testMerchantID,
				CardLast4:   intent.CardLast4(),
				ExpiryMonth: intent.ExpiryMonth,
				ExpiryYear:  intent.ExpiryYear,
				Amount:      intent.Amount,
				Currency:    intent.Currency,
				Status:      domain.StatusAuthorized,
				CreatedAt:   time.Now(),
			}, nil
		},
	}
	srv := httpserver.NewServer(paymentSvc, newTestMerchantSvc())

	tests := []struct {
		name   string
		amount int64
		status int
	}{
		{name: "minimum valid", amount: 1, status: http.StatusCreated},
		{name: "negative", amount: -1, status: http.StatusBadRequest},
		{name: "zero", amount: 0, status: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := makePaymentBody(t, "4242424242424242", "123", "USD", tt.amount, 12, 2030)
			req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+testAPIKey)
			req.Header.Set("Idempotency-Key", "idem-amount-"+tt.name)

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			assert.Equal(t, tt.status, w.Code)
		})
	}
}
