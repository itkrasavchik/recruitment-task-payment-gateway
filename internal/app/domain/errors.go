package domain

import "errors"

var (
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrPaymentAlreadyFinalized = errors.New("payment already finalized")
	ErrMerchantNotFound        = errors.New("merchant not found")
	ErrPaymentInFlight         = errors.New("payment is being processed")
)

// ValidationError wraps a business rule validation failure.
type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}
