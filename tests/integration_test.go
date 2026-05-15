//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	defaultBaseURL = "http://localhost:8080"
	apiKey         = "sk_test_merchant_key_001"

	// Test cards — last digit determines bank simulator behavior.
	cardAuthorized       = "4242424242424242" // any digit except 0/5 → authorized
	cardDeclinedFunds    = "4000000000000010" // ends in 0 → insufficient_funds
	cardInvalidLuhn      = "1234567890123456" // fails Luhn check
	cardAuthorizedMasked = "************4242"

	// Payment statuses.
	statusAuthorized = "authorized"
	statusDeclined   = "declined"

	// Error codes.
	codeValidationError = "validation_error"
	codeUnauthorized    = "unauthorized"
	codeNotFound        = "not_found"
	codeConflict        = "conflict"

	// Decline reasons.
	declineInsufficientFunds = "insufficient_funds"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func baseURL() string {
	if u := os.Getenv("API_BASE_URL"); u != "" {
		return u
	}
	return defaultBaseURL
}

// waitForAPI polls the health endpoint until the API is ready or the timeout expires.
func waitForAPI(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(baseURL() + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("API did not become healthy within 60 seconds")
}

type paymentRequest struct {
	CardNumber  string `json:"card_number"`
	ExpiryMonth int    `json:"expiry_month"`
	ExpiryYear  int    `json:"expiry_year"`
	CVV         string `json:"cvv"`
	Amount      int    `json:"amount"`
	Currency    string `json:"currency"`
}

type paymentResponse struct {
	ID                string  `json:"id"`
	Status            string  `json:"status"`
	CardLast4         string  `json:"card_last4"`
	ExpiryMonth       int     `json:"expiry_month"`
	ExpiryYear        int     `json:"expiry_year"`
	Amount            int     `json:"amount"`
	Currency          string  `json:"currency"`
	BankTransactionID *string `json:"bank_transaction_id"`
	DeclineReason     *string `json:"decline_reason"`
	CreatedAt         string  `json:"created_at"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func createPayment(t *testing.T, idempotencyKey string, req paymentRequest) (*http.Response, []byte) {
	t.Helper()
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest(http.MethodPost, baseURL()+"/payments", bytes.NewReader(body))
	require.NoError(t, err)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := httpClient.Do(httpReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, respBody
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestHealthCheck(t *testing.T) {
	waitForAPI(t)

	resp, err := httpClient.Get(baseURL() + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, "ok", body["status"])
}

func TestAuthorizedPayment(t *testing.T) {
	waitForAPI(t)

	resp, body := createPayment(t, fmt.Sprintf("integ-auth-%d", time.Now().UnixNano()), paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      5000,
		Currency:    "USD",
	})

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var payment paymentResponse
	require.NoError(t, json.Unmarshal(body, &payment))

	require.Equal(t, statusAuthorized, payment.Status)
	require.Equal(t, cardAuthorizedMasked, payment.CardLast4)
	require.Equal(t, 12, payment.ExpiryMonth)
	require.Equal(t, 2030, payment.ExpiryYear)
	require.Equal(t, 5000, payment.Amount)
	require.Equal(t, "USD", payment.Currency)
	require.NotEmpty(t, payment.ID)
	require.NotNil(t, payment.BankTransactionID)
	require.NotEmpty(t, *payment.BankTransactionID)
	require.NotEmpty(t, payment.CreatedAt)
}

func TestDeclinedPayment(t *testing.T) {
	waitForAPI(t)

	resp, body := createPayment(t, fmt.Sprintf("integ-decline-%d", time.Now().UnixNano()), paymentRequest{
		CardNumber:  cardDeclinedFunds,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      5000,
		Currency:    "USD",
	})

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var payment paymentResponse
	require.NoError(t, json.Unmarshal(body, &payment))

	require.Equal(t, statusDeclined, payment.Status)
	require.NotNil(t, payment.DeclineReason)
	require.Equal(t, declineInsufficientFunds, *payment.DeclineReason)
}

func TestIdempotentRetry(t *testing.T) {
	waitForAPI(t)

	key := fmt.Sprintf("integ-idempotent-%d", time.Now().UnixNano())
	req := paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      3000,
		Currency:    "EUR",
	}

	resp1, body1 := createPayment(t, key, req)
	require.Equal(t, http.StatusCreated, resp1.StatusCode)

	var payment1 paymentResponse
	require.NoError(t, json.Unmarshal(body1, &payment1))

	resp2, body2 := createPayment(t, key, req)
	require.Equal(t, http.StatusCreated, resp2.StatusCode)

	var payment2 paymentResponse
	require.NoError(t, json.Unmarshal(body2, &payment2))

	require.Equal(t, payment1.ID, payment2.ID, "idempotent retry should return the same payment ID")
}

func TestGetPayment(t *testing.T) {
	waitForAPI(t)

	// Create a payment first.
	_, body := createPayment(t, fmt.Sprintf("integ-get-%d", time.Now().UnixNano()), paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      1000,
		Currency:    "GBP",
	})

	var created paymentResponse
	require.NoError(t, json.Unmarshal(body, &created))

	// Retrieve it.
	req, err := http.NewRequest(http.MethodGet, baseURL()+"/payments/"+created.ID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var fetched paymentResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&fetched))

	require.Equal(t, created.ID, fetched.ID)
	require.Equal(t, statusAuthorized, fetched.Status)
	require.Equal(t, cardAuthorizedMasked, fetched.CardLast4)
	require.Equal(t, 1000, fetched.Amount)
	require.Equal(t, "GBP", fetched.Currency)
}

func TestGetPaymentNotFound(t *testing.T) {
	waitForAPI(t)

	req, err := http.NewRequest(http.MethodGet, baseURL()+"/payments/00000000-0000-0000-0000-000000000000", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errResp errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, codeNotFound, errResp.Code)
}

func TestValidationError(t *testing.T) {
	waitForAPI(t)

	resp, body := createPayment(t, fmt.Sprintf("integ-validation-%d", time.Now().UnixNano()), paymentRequest{
		CardNumber:  cardInvalidLuhn, // fails Luhn
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      100,
		Currency:    "USD",
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp errorResponse
	require.NoError(t, json.Unmarshal(body, &errResp))
	require.Equal(t, codeValidationError, errResp.Code)
}

func TestMissingAuth(t *testing.T) {
	waitForAPI(t)

	body, _ := json.Marshal(paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      100,
		Currency:    "USD",
	})

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/payments", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "no-auth")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var errResp errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, codeUnauthorized, errResp.Code)
}

func TestInvalidAuth(t *testing.T) {
	waitForAPI(t)

	body, _ := json.Marshal(paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      100,
		Currency:    "USD",
	})

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/payments", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-key")
	req.Header.Set("Idempotency-Key", "bad-auth")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var errResp errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, codeUnauthorized, errResp.Code)
}

func TestMissingIdempotencyKey(t *testing.T) {
	waitForAPI(t)

	body, _ := json.Marshal(paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      100,
		Currency:    "USD",
	})

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/payments", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	// No Idempotency-Key header.

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var errResp errorResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errResp))
	require.Equal(t, codeValidationError, errResp.Code)
}

// doPaymentRequest is like createPayment but returns the raw status code and body
// without calling t.Fatal on HTTP errors, so it's safe from concurrent goroutines.
func doPaymentRequest(idempotencyKey string, req paymentRequest) (int, []byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return 0, nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, baseURL()+"/payments", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}

// TestConcurrentSameIdempotencyKey fires N identical requests simultaneously with the
// same idempotency key. Exactly one should be processed (201 authorized); the rest
// must either return the same payment (201, idempotent hit) or 409 conflict (in-flight).
// No double-charges should occur.
func TestConcurrentSameIdempotencyKey(t *testing.T) {
	waitForAPI(t)

	const concurrency = 10
	key := fmt.Sprintf("integ-concurrent-%d", time.Now().UnixNano())
	req := paymentRequest{
		CardNumber:  cardAuthorized,
		ExpiryMonth: 12,
		ExpiryYear:  2030,
		CVV:         "123",
		Amount:      7777,
		Currency:    "USD",
	}

	type result struct {
		statusCode int
		body       []byte
		err        error
	}

	results := make([]result, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := range concurrency {
		go func(idx int) {
			defer wg.Done()
			sc, body, err := doPaymentRequest(key, req)
			results[idx] = result{sc, body, err}
		}(i)
	}
	wg.Wait()

	var paymentIDs []string
	var created, conflict int
	for i, r := range results {
		require.NoError(t, r.err, "request %d failed", i)

		switch r.statusCode {
		case http.StatusCreated:
			created++
			var p paymentResponse
			require.NoError(t, json.Unmarshal(r.body, &p), "request %d", i)
			require.Equal(t, statusAuthorized, p.Status, "request %d", i)
			paymentIDs = append(paymentIDs, p.ID)
		case http.StatusConflict:
			conflict++
			var e errorResponse
			require.NoError(t, json.Unmarshal(r.body, &e), "request %d", i)
			require.Equal(t, codeConflict, e.Code, "request %d", i)
		default:
			t.Errorf("request %d: unexpected status %d: %s", i, r.statusCode, string(r.body))
		}
	}

	// At least one request must succeed.
	require.GreaterOrEqual(t, created, 1, "at least one request should succeed with 201")

	// All successful responses must return the same payment ID (no double-charge).
	if len(paymentIDs) > 1 {
		for _, id := range paymentIDs[1:] {
			require.Equal(t, paymentIDs[0], id, "all 201 responses must return the same payment ID")
		}
	}

	t.Logf("concurrency results: %d created, %d conflict (out of %d)", created, conflict, concurrency)
}

// TestConcurrentDistinctPayments fires N requests with different idempotency keys
// concurrently. All should succeed independently — verifying no global locking issues.
func TestConcurrentDistinctPayments(t *testing.T) {
	waitForAPI(t)

	const concurrency = 20

	type result struct {
		statusCode int
		body       []byte
		err        error
	}

	results := make([]result, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := range concurrency {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("integ-distinct-%d-%d", time.Now().UnixNano(), idx)
			req := paymentRequest{
				CardNumber:  cardAuthorized,
				ExpiryMonth: 12,
				ExpiryYear:  2030,
				CVV:         "123",
				Amount:      1000 + idx,
				Currency:    "USD",
			}
			sc, body, err := doPaymentRequest(key, req)
			results[idx] = result{sc, body, err}
		}(i)
	}
	wg.Wait()

	ids := make(map[string]bool)
	for i, r := range results {
		require.NoError(t, r.err, "request %d failed", i)
		require.Equal(t, http.StatusCreated, r.statusCode, "request %d: %s", i, string(r.body))

		var p paymentResponse
		require.NoError(t, json.Unmarshal(r.body, &p), "request %d", i)
		require.Equal(t, statusAuthorized, p.Status, "request %d", i)
		require.False(t, ids[p.ID], "request %d: duplicate payment ID %s", i, p.ID)
		ids[p.ID] = true
	}

	require.Len(t, ids, concurrency, "each request should create a unique payment")
}
