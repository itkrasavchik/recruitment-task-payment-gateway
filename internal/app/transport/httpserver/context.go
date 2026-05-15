package httpserver

import (
	"context"
	"fmt"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

type contextKey string

const (
	merchantKey  contextKey = "merchant"
	requestIDKey contextKey = "request_id"
)

func withMerchant(ctx context.Context, m *domain.Merchant) context.Context {
	return context.WithValue(ctx, merchantKey, m)
}

func getMerchant(ctx context.Context) *domain.Merchant {
	v := ctx.Value(merchantKey)
	if v == nil {
		return nil
	}
	m, ok := v.(*domain.Merchant)
	if !ok {
		// Panic is intentional: context keys are unexported, so a type mismatch
		// is always a programming error in this package — not a runtime condition
		// we can recover from. Failing fast surfaces the bug immediately.
		panic(fmt.Sprintf("unexpected type in context for key %q: %T", merchantKey, v))
	}
	return m
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func getRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey)
	if v == nil {
		return ""
	}
	id, ok := v.(string)
	if !ok {
		// See comment in getMerchant — same rationale.
		panic(fmt.Sprintf("unexpected type in context for key %q: %T", requestIDKey, v))
	}
	return id
}
