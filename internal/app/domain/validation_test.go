package domain //nolint:testpackage // testing unexported validators directly

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCardNumber(t *testing.T) {
	tests := []struct {
		name    string
		number  string
		wantErr string
	}{
		{name: "valid visa", number: "4242424242424242"},
		{name: "valid 13 digits", number: "4222222222222"},
		{name: "too short", number: "123456789012", wantErr: "between 13 and 19 digits"},
		{name: "too long", number: "12345678901234567890", wantErr: "between 13 and 19 digits"},
		{name: "non-digits", number: "4242-4242-4242-4242", wantErr: "only digits"},
		{name: "letters", number: "42424242abcd4242", wantErr: "only digits"},
		{name: "fails luhn", number: "4242424242424241", wantErr: "Luhn check"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCardNumber(tt.number)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateExpiry(t *testing.T) {
	tests := []struct {
		name    string
		month   int
		year    int
		wantErr string
	}{
		{name: "valid future", month: 12, year: 2030},
		{name: "month too low", month: 0, year: 2030, wantErr: "between 1 and 12"},
		{name: "month too high", month: 13, year: 2030, wantErr: "between 1 and 12"},
		{name: "expired year", month: 12, year: 2020, wantErr: "expired"},
		{name: "expired month same year", month: 1, year: 2024, wantErr: "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExpiry(tt.month, tt.year)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCVV(t *testing.T) {
	tests := []struct {
		name    string
		cvv     string
		wantErr string
	}{
		{name: "valid 3 digits", cvv: "123"},
		{name: "valid 4 digits", cvv: "1234"},
		{name: "too short", cvv: "12", wantErr: "3 or 4 digits"},
		{name: "too long", cvv: "12345", wantErr: "3 or 4 digits"},
		{name: "non-digits", cvv: "12a", wantErr: "only digits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCVV(tt.cvv)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAmount(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		wantErr string
	}{
		{name: "valid", amount: 100},
		{name: "zero", amount: 0, wantErr: "greater than 0"},
		{name: "negative", amount: -1, wantErr: "greater than 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAmount(tt.amount)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateCurrency(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		wantErr  string
	}{
		{name: "USD", currency: "USD"},
		{name: "EUR", currency: "EUR"},
		{name: "lowercase usd", currency: "usd"},
		{name: "invalid", currency: "XYZ", wantErr: "unsupported currency"},
		{name: "empty", currency: "", wantErr: "unsupported currency"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCurrency(tt.currency)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
