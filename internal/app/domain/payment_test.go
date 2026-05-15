package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

func TestNewPaymentIntent_Valid(t *testing.T) {
	intent, err := domain.NewPaymentIntent(
		"merchant-1", "idem-1", "4242424242424242",
		12, 2030, "123", 5000, "USD",
	)

	require.NoError(t, err)
	assert.Equal(t, "merchant-1", intent.MerchantID)
	assert.Equal(t, "idem-1", intent.IdempotencyKey)
	assert.Equal(t, "4242424242424242", intent.CardNumber)
	assert.Equal(t, 12, intent.ExpiryMonth)
	assert.Equal(t, 2030, intent.ExpiryYear)
	assert.Equal(t, "123", intent.CVV)
	assert.Equal(t, int64(5000), intent.Amount)
	assert.Equal(t, "USD", intent.Currency)
}

func TestNewPaymentIntent_UppercasesCurrency(t *testing.T) {
	intent, err := domain.NewPaymentIntent(
		"m", "i", "4242424242424242",
		12, 2030, "123", 100, "eur",
	)

	require.NoError(t, err)
	assert.Equal(t, "EUR", intent.Currency)
}

func TestNewPaymentIntent_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		cardNumber string
		month      int
		year       int
		cvv        string
		amount     int64
		currency   string
		wantErr    string
	}{
		{
			name: "bad card", cardNumber: "1234567890123456",
			month: 12, year: 2030, cvv: "123", amount: 100, currency: "USD",
			wantErr: "Luhn",
		},
		{
			name: "bad expiry", cardNumber: "4242424242424242",
			month: 13, year: 2030, cvv: "123", amount: 100, currency: "USD",
			wantErr: "between 1 and 12",
		},
		{
			name: "bad cvv", cardNumber: "4242424242424242",
			month: 12, year: 2030, cvv: "ab", amount: 100, currency: "USD",
			wantErr: "3 or 4 digits",
		},
		{
			name: "bad amount", cardNumber: "4242424242424242",
			month: 12, year: 2030, cvv: "123", amount: 0, currency: "USD",
			wantErr: "greater than 0",
		},
		{
			name: "bad currency", cardNumber: "4242424242424242",
			month: 12, year: 2030, cvv: "123", amount: 100, currency: "XYZ",
			wantErr: "unsupported currency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewPaymentIntent(
				"m", "i", tt.cardNumber,
				tt.month, tt.year, tt.cvv, tt.amount, tt.currency,
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)

			var valErr *domain.ValidationError
			assert.ErrorAs(t, err, &valErr)
		})
	}
}

func TestPaymentIntent_CardLast4(t *testing.T) {
	intent, err := domain.NewPaymentIntent(
		"m", "i", "4242424242424242",
		12, 2030, "123", 100, "USD",
	)
	require.NoError(t, err)
	assert.Equal(t, "4242", intent.CardLast4())
}

func TestValidationError_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("test error")
	ve := &domain.ValidationError{Err: inner}

	assert.Equal(t, "test error", ve.Error())
	assert.Equal(t, inner, ve.Unwrap())
}
