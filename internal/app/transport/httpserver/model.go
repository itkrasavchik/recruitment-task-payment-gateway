package httpserver

import (
	"time"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

type CreatePaymentRequest struct {
	CardNumber  string `json:"card_number"`
	ExpiryMonth int    `json:"expiry_month"`
	ExpiryYear  int    `json:"expiry_year"`
	CVV         string `json:"cvv"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
}

type PaymentResponse struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	CardLast4         string `json:"card_last4"`
	ExpiryMonth       int    `json:"expiry_month"`
	ExpiryYear        int    `json:"expiry_year"`
	Amount            int64  `json:"amount"`
	Currency          string `json:"currency"`
	DeclineReason     string `json:"decline_reason,omitempty"`
	BankTransactionID string `json:"bank_transaction_id,omitempty"`
	CreatedAt         string `json:"created_at"`
}

func toPaymentResponse(p *domain.Payment) PaymentResponse {
	resp := PaymentResponse{
		ID:          p.ID,
		Status:      p.Status.String(),
		CardLast4:   "************" + p.CardLast4,
		ExpiryMonth: p.ExpiryMonth,
		ExpiryYear:  p.ExpiryYear,
		Amount:      p.Amount,
		Currency:    p.Currency,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
	}
	if p.DeclineReason != nil {
		resp.DeclineReason = *p.DeclineReason
	}
	if p.BankTransactionID != nil {
		resp.BankTransactionID = *p.BankTransactionID
	}
	return resp
}
