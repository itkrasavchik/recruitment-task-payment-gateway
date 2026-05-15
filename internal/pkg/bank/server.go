package bank

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// NewHandler returns an [http.Handler] that wraps the bank Simulator.
func NewHandler(sim *Simulator) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /authorize", func(w http.ResponseWriter, r *http.Request) {
		const maxBodySize = 1 << 20 // 1 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		var req Request
		if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		resp, procErr := sim.ProcessPayment(r.Context(), req)
		if procErr != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
			slog.Error("failed to encode response", "error", encErr)
		}
	})

	return mux
}
