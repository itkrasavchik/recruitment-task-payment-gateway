package bank

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/metrics"
)

// Circuit breaker configuration.
const (
	cbFailureThreshold = 5
	cbOpenDuration     = 10 * time.Second
)

// Retry configuration.
const (
	maxAttempts  = 3
	baseDelay    = 100 * time.Millisecond
	backoffMulti = 2
)

type circuitState int

const (
	stateClosed circuitState = iota
	stateOpen
	stateHalfOpen
)

var errCircuitOpen = errors.New("circuit breaker is open, failing fast")

// transientError marks errors that should trip the circuit breaker and be retried.
type transientError struct {
	err error
}

func (e *transientError) Error() string { return e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

func isTransient(err error) bool {
	var te *transientError
	return errors.As(err, &te)
}

// HTTPClient implements the Client interface by calling the bank simulator
// over HTTP. It includes a circuit breaker and retry with exponential backoff.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client

	mu               sync.Mutex
	state            circuitState
	consecutiveFails int
	openUntil        time.Time
}

// NewHTTPClient creates a new HTTP-based bank client.
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ProcessPayment sends an authorization request to the bank simulator over HTTP.
func (c *HTTPClient) ProcessPayment(ctx context.Context, req Request) (Response, error) {
	var lastErr error

	for attempt := range maxAttempts {
		if cbErr := c.checkCircuitBreaker(); cbErr != nil {
			return Response{}, cbErr
		}

		resp, doErr := c.doRequest(ctx, req)
		if doErr == nil {
			c.recordSuccess()
			return resp, nil
		}

		lastErr = doErr

		// Only transient errors (5xx, network) trip the circuit breaker and are retried.
		// 4xx errors are returned immediately without affecting the breaker.
		if !isTransient(doErr) {
			return Response{}, doErr
		}

		c.recordFailure()

		// Don't retry on context cancellation/deadline.
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}

		// Don't retry on the last attempt.
		if attempt < maxAttempts-1 {
			delay := c.retryDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return Response{}, ctx.Err()
			}
		}
	}

	return Response{}, fmt.Errorf("bank request failed after %d attempts: %w", maxAttempts, lastErr)
}

func (c *HTTPClient) doRequest(ctx context.Context, req Request) (Response, error) {
	start := time.Now()

	body, err := json.Marshal(req)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/authorize", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq) //nolint:gosec // URL comes from trusted config, not user input
	if err != nil {
		metrics.BankRequestDuration.Observe(time.Since(start).Seconds())
		metrics.BankRequestsTotal.WithLabelValues("transient_error").Inc()
		return Response{}, &transientError{err: fmt.Errorf("execute request: %w", err)}
	}
	defer httpResp.Body.Close()

	// 5xx → transient error, will be retried and trips circuit breaker.
	if httpResp.StatusCode >= http.StatusInternalServerError {
		metrics.BankRequestDuration.Observe(time.Since(start).Seconds())
		metrics.BankRequestsTotal.WithLabelValues("transient_error").Inc()
		return Response{}, &transientError{err: fmt.Errorf("bank returned status %d", httpResp.StatusCode)}
	}

	// 4xx → client error, don't retry, don't trip circuit breaker.
	if httpResp.StatusCode >= http.StatusBadRequest {
		metrics.BankRequestDuration.Observe(time.Since(start).Seconds())
		metrics.BankRequestsTotal.WithLabelValues("client_error").Inc()
		return Response{}, fmt.Errorf("bank returned status %d", httpResp.StatusCode)
	}

	const maxResponseSize = 1 << 20 // 1 MB
	var resp Response
	if decErr := json.NewDecoder(io.LimitReader(httpResp.Body, maxResponseSize)).Decode(&resp); decErr != nil {
		metrics.BankRequestDuration.Observe(time.Since(start).Seconds())
		metrics.BankRequestsTotal.WithLabelValues("client_error").Inc()
		return Response{}, fmt.Errorf("decode response: %w", decErr)
	}

	metrics.BankRequestDuration.Observe(time.Since(start).Seconds())
	metrics.BankRequestsTotal.WithLabelValues("success").Inc()
	return resp, nil
}

// checkCircuitBreaker returns an error if the circuit is open.
func (c *HTTPClient) checkCircuitBreaker() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	//nolint:exhaustive // closed and half-open both allow requests through
	switch c.state {
	case stateOpen:
		if time.Now().Before(c.openUntil) {
			return errCircuitOpen
		}
		// Cooldown elapsed → allow one probe.
		c.state = stateHalfOpen
		metrics.BankCircuitBreakerState.Set(float64(stateHalfOpen))
		return nil
	default:
		return nil
	}
}

func (c *HTTPClient) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFails = 0
	c.state = stateClosed
	metrics.BankCircuitBreakerState.Set(float64(stateClosed))
}

func (c *HTTPClient) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFails++

	if c.state == stateHalfOpen || c.consecutiveFails >= cbFailureThreshold {
		c.state = stateOpen
		c.openUntil = time.Now().Add(cbOpenDuration)
		metrics.BankCircuitBreakerState.Set(float64(stateOpen))
	}
}

// retryDelay returns the delay for a given attempt with jitter (±50%).
func (c *HTTPClient) retryDelay(attempt int) time.Duration {
	delay := baseDelay
	for range attempt {
		delay *= backoffMulti
	}

	// Apply ±50% jitter. Using math/rand is fine here — this is not security-sensitive.
	const jitterBase = 0.5
	//nolint:gosec // jitter does not need cryptographic randomness
	jitter := jitterBase + rand.Float64()
	return time.Duration(float64(delay) * jitter)
}
