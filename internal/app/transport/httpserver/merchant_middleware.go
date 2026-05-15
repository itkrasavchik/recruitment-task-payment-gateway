package httpserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/evt/recruitment-task-payment-gateway/internal/app/common/server"
	"github.com/evt/recruitment-task-payment-gateway/internal/app/domain"
)

func merchantAuthMiddleware(
	merchantSvc MerchantService,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				server.Unauthorized("missing Authorization header").Write(w)
				return
			}

			apiKey := strings.TrimPrefix(authHeader, "Bearer ")
			if apiKey == authHeader {
				server.Unauthorized("invalid Authorization format, expected Bearer <key>").Write(w)
				return
			}

			merchant, err := merchantSvc.Authenticate(r.Context(), apiKey)
			if err != nil {
				if errors.Is(err, domain.ErrMerchantNotFound) {
					server.Unauthorized("invalid API key").Write(w)
					return
				}
				server.InternalError("authentication failed").Write(w)
				return
			}

			ctx := withMerchant(r.Context(), merchant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
