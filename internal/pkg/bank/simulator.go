package bank

import (
	"context"

	"github.com/google/uuid"
)

type Simulator struct{}

func NewSimulator() *Simulator {
	return &Simulator{}
}

func (s *Simulator) ProcessPayment(ctx context.Context, req Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}

	last := req.CardNumber[len(req.CardNumber)-1]

	switch last {
	case '0':
		return Response{
			Authorized:    false,
			TransactionID: uuid.New().String(),
			DeclineReason: "insufficient_funds",
		}, nil
	case '5':
		return Response{
			Authorized:    false,
			TransactionID: uuid.New().String(),
			DeclineReason: "expired_card",
		}, nil
	default:
		return Response{
			Authorized:    true,
			TransactionID: uuid.New().String(),
		}, nil
	}
}
