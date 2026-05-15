package bank_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/evt/recruitment-task-payment-gateway/internal/pkg/bank"
)

func TestHandler_Health(t *testing.T) {
	handler := bank.NewHandler(bank.NewSimulator())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_Authorize_Success(t *testing.T) {
	handler := bank.NewHandler(bank.NewSimulator())
	body, _ := json.Marshal(bank.Request{
		CardNumber: "4242424242424242",
		Amount:     1000,
		Currency:   "USD",
	})
	req := httptest.NewRequest(http.MethodPost, "/authorize", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp bank.Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.True(t, resp.Authorized)
	assert.NotEmpty(t, resp.TransactionID)
}

func TestHandler_Authorize_Declined(t *testing.T) {
	handler := bank.NewHandler(bank.NewSimulator())
	body, _ := json.Marshal(bank.Request{
		CardNumber: "4000000000000010",
		Amount:     1000,
		Currency:   "USD",
	})
	req := httptest.NewRequest(http.MethodPost, "/authorize", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp bank.Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.False(t, resp.Authorized)
	assert.Equal(t, "insufficient_funds", resp.DeclineReason)
}

func TestHandler_Authorize_InvalidBody(t *testing.T) {
	handler := bank.NewHandler(bank.NewSimulator())
	req := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Authorize_CancelledContext(t *testing.T) {
	handler := bank.NewHandler(bank.NewSimulator())
	body, _ := json.Marshal(bank.Request{
		CardNumber: "4242424242424242",
		Amount:     1000,
		Currency:   "USD",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodPost, "/authorize", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
