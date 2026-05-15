package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorCode is a machine-readable error identifier.
type ErrorCode string

const (
	CodeValidationError ErrorCode = "validation_error"
	CodeUnauthorized    ErrorCode = "unauthorized"
	CodeNotFound        ErrorCode = "not_found"
	CodeConflict        ErrorCode = "conflict"
	CodeInternalError   ErrorCode = "internal_error"
	CodeBankError       ErrorCode = "bank_error"
)

// HTTPError represents an API error that can write itself to an [http.ResponseWriter].
type HTTPError struct {
	Status  int       `json:"-"`
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

// NewError creates an HTTPError with the given status, code, and message.
func NewError(status int, code ErrorCode, message string) HTTPError {
	return HTTPError{Status: status, Code: code, Message: message}
}

// Write serializes the error as JSON to the response writer.
func (e HTTPError) Write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.Status)
	if err := json.NewEncoder(w).Encode(e); err != nil {
		slog.Error("failed to encode error response", "error", err) //nolint:sloglint // no context in scope
	}
}

// Unauthorized returns a 401 error.
func Unauthorized(message string) HTTPError {
	return NewError(http.StatusUnauthorized, CodeUnauthorized, message)
}

// ValidationError returns a 400 validation error.
func ValidationError(message string) HTTPError {
	return NewError(http.StatusBadRequest, CodeValidationError, message)
}

// NotFound returns a 404 error.
func NotFound(message string) HTTPError {
	return NewError(http.StatusNotFound, CodeNotFound, message)
}

// Conflict returns a 409 error.
func Conflict(message string) HTTPError {
	return NewError(http.StatusConflict, CodeConflict, message)
}

// InternalError returns a 500 error.
func InternalError(message string) HTTPError {
	return NewError(http.StatusInternalServerError, CodeInternalError, message)
}

// BankError returns a 502 error.
func BankError(message string) HTTPError {
	return NewError(http.StatusBadGateway, CodeBankError, message)
}
