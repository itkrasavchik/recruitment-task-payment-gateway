package domain

import (
	"strings"
	"time"
)

type PaymentStatus string

func (s PaymentStatus) String() string {
	return string(s)
}

const (
	StatusPending    PaymentStatus = "pending"
	StatusAuthorized PaymentStatus = "authorized"
	StatusDeclined   PaymentStatus = "declined"
	StatusFailed     PaymentStatus = "failed"
)

// PaymentIntent represents a validated payment request before bank processing.
// Created exclusively via NewPaymentIntent, which enforces all business rules.
// Passed by value (not pointer) intentionally: it is small, immutable after
// construction, and should not be modified by downstream layers.
type PaymentIntent struct {
	MerchantID     string
	IdempotencyKey string
	CardNumber     string
	ExpiryMonth    int
	ExpiryYear     int
	CVV            string
	Amount         int64
	Currency       string
}

// NewPaymentIntent validates input and returns a PaymentIntent.
// Returns *ValidationError if any business rule is violated.
func NewPaymentIntent(
	merchantID, idempotencyKey, cardNumber string,
	expiryMonth, expiryYear int,
	cvv string,
	amount int64,
	currency string,
) (PaymentIntent, error) {
	if err := validateCardNumber(cardNumber); err != nil {
		return PaymentIntent{}, &ValidationError{Err: err}
	}
	if err := validateExpiry(expiryMonth, expiryYear); err != nil {
		return PaymentIntent{}, &ValidationError{Err: err}
	}
	if err := validateCVV(cvv); err != nil {
		return PaymentIntent{}, &ValidationError{Err: err}
	}
	if err := validateAmount(amount); err != nil {
		return PaymentIntent{}, &ValidationError{Err: err}
	}
	if err := validateCurrency(currency); err != nil {
		return PaymentIntent{}, &ValidationError{Err: err}
	}

	return PaymentIntent{
		MerchantID:     merchantID,
		IdempotencyKey: idempotencyKey,
		CardNumber:     cardNumber,
		ExpiryMonth:    expiryMonth,
		ExpiryYear:     expiryYear,
		CVV:            cvv,
		Amount:         amount,
		Currency:       strings.ToUpper(currency),
	}, nil
}

// CardLast4 returns the last 4 digits of the card number.
func (i PaymentIntent) CardLast4() string {
	return i.CardNumber[len(i.CardNumber)-4:]
}

// Payment represents a processed payment with bank result and persistence metadata.
// Returned as a pointer: it is a larger struct with optional fields (pointer members),
// and callers need nil semantics on the error path.
type Payment struct {
	ID                string
	MerchantID        string
	IdempotencyKey    string
	CardLast4         string
	ExpiryMonth       int
	ExpiryYear        int
	Amount            int64
	Currency          string
	Status            PaymentStatus
	DeclineReason     *string
	BankTransactionID *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IsPending reports whether the payment is still awaiting a bank result.
func (p *Payment) IsPending() bool {
	return p.Status == StatusPending
}

// MarkAuthorized transitions the payment to authorized with the bank transaction ID.
func (p *Payment) MarkAuthorized(transactionID string) {
	p.Status = StatusAuthorized
	p.BankTransactionID = &transactionID
}

// MarkDeclined transitions the payment to declined with the reason and bank transaction ID.
func (p *Payment) MarkDeclined(reason, transactionID string) {
	p.Status = StatusDeclined
	p.DeclineReason = &reason
	p.BankTransactionID = &transactionID
}

// MarkFailed transitions the payment to failed with the given reason.
func (p *Payment) MarkFailed(reason string) {
	p.Status = StatusFailed
	p.DeclineReason = &reason
}
