package bank_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evt/recruitment-task-payment-gateway/internal/pkg/bank"
)

func TestSimulator_Authorized(t *testing.T) {
	sim := bank.NewSimulator()
	resp, err := sim.ProcessPayment(context.Background(), bank.Request{
		CardNumber: "4242424242424242",
		Amount:     1000,
		Currency:   "USD",
	})

	require.NoError(t, err)
	assert.True(t, resp.Authorized)
	assert.NotEmpty(t, resp.TransactionID)
	assert.Empty(t, resp.DeclineReason)
}

func TestSimulator_DeclinedInsufficientFunds(t *testing.T) {
	sim := bank.NewSimulator()
	resp, err := sim.ProcessPayment(context.Background(), bank.Request{
		CardNumber: "4000000000000010",
		Amount:     1000,
		Currency:   "USD",
	})

	require.NoError(t, err)
	assert.False(t, resp.Authorized)
	assert.Equal(t, "insufficient_funds", resp.DeclineReason)
	assert.NotEmpty(t, resp.TransactionID)
}

func TestSimulator_DeclinedExpiredCard(t *testing.T) {
	sim := bank.NewSimulator()
	resp, err := sim.ProcessPayment(context.Background(), bank.Request{
		CardNumber: "4000000000000035",
		Amount:     1000,
		Currency:   "USD",
	})

	require.NoError(t, err)
	assert.False(t, resp.Authorized)
	assert.Equal(t, "expired_card", resp.DeclineReason)
	assert.NotEmpty(t, resp.TransactionID)
}

func TestSimulator_CancelledContext(t *testing.T) {
	sim := bank.NewSimulator()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := sim.ProcessPayment(ctx, bank.Request{
		CardNumber: "4242424242424242",
	})

	require.Error(t, err)
}

func TestSimulator_UniqueTransactionIDs(t *testing.T) {
	sim := bank.NewSimulator()
	req := bank.Request{CardNumber: "4242424242424242", Amount: 100, Currency: "USD"}

	resp1, err1 := sim.ProcessPayment(context.Background(), req)
	resp2, err2 := sim.ProcessPayment(context.Background(), req)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, resp1.TransactionID, resp2.TransactionID)
}
