package domain

import (
	"errors"
	"strings"
	"time"
	"unicode"
)

const (
	cardNumberMinLen = 13
	cardNumberMaxLen = 19
	luhnDoubleDigit  = 9
)

var validCurrencies = map[string]bool{
	"USD": true, "EUR": true, "GBP": true,
	"JPY": true, "CHF": true, "CAD": true,
	"AUD": true, "NZD": true, "SEK": true,
	"NOK": true, "DKK": true, "SGD": true,
	"HKD": true, "PLN": true, "CZK": true,
}

func isValidCurrency(currency string) bool {
	return validCurrencies[currency]
}

// validateCardNumber checks length, numeric-only, and Luhn.
func validateCardNumber(number string) error {
	if len(number) < cardNumberMinLen || len(number) > cardNumberMaxLen {
		return errors.New("card number must be between 13 and 19 digits")
	}
	for _, r := range number {
		if !unicode.IsDigit(r) {
			return errors.New("card number must contain only digits")
		}
	}
	if !luhnCheck(number) {
		return errors.New("card number failed Luhn check")
	}
	return nil
}

func luhnCheck(number string) bool {
	sum := 0
	alt := false
	for i := range len(number) {
		n := int(number[len(number)-1-i] - '0')
		if alt {
			n *= 2
			if n > luhnDoubleDigit {
				n -= luhnDoubleDigit
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// validateExpiry checks month range and that the card is not expired.
func validateExpiry(month, year int) error {
	if month < 1 || month > 12 {
		return errors.New("expiry month must be between 1 and 12")
	}
	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())
	if year < currentYear || (year == currentYear && month < currentMonth) {
		return errors.New("card has expired")
	}
	return nil
}

// validateCVV checks 3 or 4 numeric digits.
func validateCVV(cvv string) error {
	if len(cvv) < 3 || len(cvv) > 4 {
		return errors.New("CVV must be 3 or 4 digits")
	}
	for _, r := range cvv {
		if !unicode.IsDigit(r) {
			return errors.New("CVV must contain only digits")
		}
	}
	return nil
}

// validateAmount checks that the amount is positive.
func validateAmount(amount int64) error {
	if amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	return nil
}

// validateCurrency checks against the allowed ISO 4217 set.
func validateCurrency(currency string) error {
	if !isValidCurrency(strings.ToUpper(currency)) {
		return errors.New("unsupported currency: " + currency)
	}
	return nil
}
