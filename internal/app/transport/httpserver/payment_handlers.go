package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/common/server"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

type PaymentHandler struct {
	paymentSvc PaymentService
}

func NewPaymentHandler(paymentSvc PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentSvc: paymentSvc}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	merchant := getMerchant(r.Context())
	if merchant == nil {
		server.Unauthorized("merchant not found in context").Write(w)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		server.ValidationError("Idempotency-Key header is required").Write(w)
		return
	}

	const maxBodySize = 1 << 20 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.ValidationError("invalid request body").Write(w)
		return
	}

	intent, err := domain.NewPaymentIntent(
		merchant.ID, idempotencyKey, req.CardNumber,
		req.ExpiryMonth, req.ExpiryYear,
		req.CVV, req.Amount, req.Currency,
	)
	if err != nil {
		var validationErr *domain.ValidationError
		if errors.As(err, &validationErr) {
			server.ValidationError(validationErr.Error()).Write(w)
			return
		}
		server.InternalError("unexpected error").Write(w)
		return
	}

	payment, err := h.paymentSvc.ProcessPayment(r.Context(), intent)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentInFlight) {
			server.Conflict("payment is being processed").Write(w)
			return
		}
		server.BankError("payment processing failed").Write(w)
		return
	}

	server.WriteJSON(w, http.StatusCreated, toPaymentResponse(payment))
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	merchant := getMerchant(r.Context())
	if merchant == nil {
		server.Unauthorized("merchant not found in context").Write(w)
		return
	}

	paymentID := chi.URLParam(r, "paymentID")
	if paymentID == "" {
		server.ValidationError("payment ID is required").Write(w)
		return
	}

	payment, err := h.paymentSvc.GetPayment(r.Context(), merchant.ID, paymentID)
	if err != nil {
		if errors.Is(err, domain.ErrPaymentNotFound) {
			server.NotFound("payment not found").Write(w)
			return
		}
		server.InternalError("failed to get payment").Write(w)
		return
	}

	server.WriteJSON(w, http.StatusOK, toPaymentResponse(payment))
}
