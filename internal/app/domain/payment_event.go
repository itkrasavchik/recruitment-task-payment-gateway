package domain

import (
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventPaymentCreated EventType = "payment_created"
	EventBankAuthorized EventType = "bank_authorized"
	EventBankDeclined   EventType = "bank_declined"
	EventBankFailed     EventType = "bank_failed"
)

type PaymentEvent struct {
	ID        string
	PaymentID string
	EventType EventType
	Data      map[string]any
	CreatedAt time.Time
}

func NewPaymentEvent(paymentID string, eventType EventType, data map[string]any) *PaymentEvent {
	return &PaymentEvent{
		ID:        uuid.New().String(),
		PaymentID: paymentID,
		EventType: eventType,
		Data:      data,
		CreatedAt: time.Now(),
	}
}
