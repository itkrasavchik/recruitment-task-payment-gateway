package bank

import "context"

type Request struct {
	CardNumber  string `json:"card_number"`
	ExpiryMonth int    `json:"expiry_month"`
	ExpiryYear  int    `json:"expiry_year"`
	CVV         string `json:"cvv"`
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
}

type Response struct {
	Authorized    bool   `json:"authorized"`
	TransactionID string `json:"transaction_id"`
	DeclineReason string `json:"decline_reason,omitempty"`
}

type Client interface {
	ProcessPayment(ctx context.Context, req Request) (Response, error)
}
